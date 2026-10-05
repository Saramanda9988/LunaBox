//go:build windows

package audioutils

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Opt in with LUNABOX_AUDIO_INTEGRATION=1 on a Windows host with an output
// device. Only the test executable's silent waveOut session is modified.
func TestProcessMuteAudioLifecycle(t *testing.T) {
	if os.Getenv("LUNABOX_AUDIO_INTEGRATION") != "1" {
		t.Skip("requires an audio output device; set LUNABOX_AUDIO_INTEGRATION=1")
	}
	for _, killed := range []bool{false, true} {
		t.Run(fmt.Sprintf("killed=%v", killed), func(t *testing.T) {
			child, input := startSilentAudioChild(t)
			pid := uint32(child.Process.Pid)
			waitForTestAudio(t, pid)
			if matched, err := SetProcessMuted(pid, true); !matched || err != nil {
				t.Fatalf("mute helper: matched=%v err=%v", matched, err)
			}
			if !testAudioMuted(t, pid) {
				t.Fatal("helper should be muted before exit")
			}
			if killed {
				if err := child.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				fmt.Fprintln(input, "stop")
			}
			child.Wait()
			// Deliberately let the process's streams disappear before restoring.
			time.Sleep(100 * time.Millisecond)
			if matched, err := SetProcessMuted(pid, false); !matched || err != nil {
				t.Errorf("restore exited helper: matched=%v err=%v", matched, err)
			}
			restarted, _ := startSilentAudioChild(t)
			restartedPID := uint32(restarted.Process.Pid)
			waitForTestAudio(t, restartedPID)
			if testAudioMuted(t, restartedPID) {
				t.Error("restarted executable inherited background mute")
			}
		})
	}
}

func TestProcessMuteForegroundAndRepeatedMute(t *testing.T) {
	if os.Getenv("LUNABOX_AUDIO_INTEGRATION") != "1" {
		t.Skip("requires an audio output device; set LUNABOX_AUDIO_INTEGRATION=1")
	}
	child, _ := startSilentAudioChild(t)
	pid := uint32(child.Process.Pid)
	waitForTestAudio(t, pid)
	// Seed a mute outside SetProcessMuted, as an older version could leave behind.
	_, err := withTestAudioVolume(pid, func(volume *iSimpleAudioVolume) {
		if err := setAudioVolumeMuted(volume, true); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if matched, err := SetProcessMuted(pid, false); !matched || err != nil {
		t.Fatalf("foreground restoration: matched=%v err=%v", matched, err)
	}
	if testAudioMuted(t, pid) {
		t.Fatal("foreground restoration left helper muted")
	}
	for i := 0; i < 3; i++ {
		if matched, err := SetProcessMuted(pid, true); !matched || err != nil {
			t.Fatalf("repeated mute: matched=%v err=%v", matched, err)
		}
	}
	if !testAudioMuted(t, pid) {
		t.Fatal("repeated mute did not silence helper")
	}
	if matched, err := SetProcessMuted(pid, false); !matched || err != nil {
		t.Fatalf("restore live helper: matched=%v err=%v", matched, err)
	}
	if testAudioMuted(t, pid) {
		t.Fatal("restoration left helper muted")
	}
}

func TestRetainedAudioRestorationKeepsOnlyFailedSessions(t *testing.T) {
	var restored, released int
	var retryCalls, retryReleased int
	unexpectedMute := false
	makeSession := func(setMute func(uintptr) uintptr, releaseCount *int) retainedAudioSession {
		releaseCallback := syscall.NewCallback(func(uintptr) uintptr {
			*releaseCount++
			return 0
		})
		return retainedAudioSession{
			control: &iAudioSessionControl2{vtbl: &iAudioSessionControl2Vtbl{iUnknownVtbl: iUnknownVtbl{release: releaseCallback}}},
			volume: &iSimpleAudioVolume{vtbl: &iSimpleAudioVolumeVtbl{
				iUnknownVtbl: iUnknownVtbl{release: releaseCallback},
				setMute: syscall.NewCallback(func(_ uintptr, muted uintptr, _ uintptr) uintptr {
					unexpectedMute = unexpectedMute || muted != 0
					return setMute(muted)
				}),
			}},
		}
	}
	state := processMuteState{sessions: map[uint32]map[string]retainedAudioSession{
		42: {
			"restored": makeSession(func(uintptr) uintptr { restored++; return 0 }, &released),
			"retry": makeSession(func(uintptr) uintptr {
				retryCalls++
				if retryCalls == 1 {
					return 0x80004005
				}
				return 0
			}, &retryReleased),
		},
	}}
	matched, err := state.setProcessMuted(42, false)
	if !matched || err == nil || len(state.sessions[42]) != 1 || released != 2 || retryReleased != 0 {
		t.Fatalf("partial restoration lost references or success: matched=%v err=%v retained=%d releases=%d retry releases=%d", matched, err, len(state.sessions[42]), released, retryReleased)
	}
	matched, err = state.setProcessMuted(42, false)
	if !matched || err != nil || len(state.sessions) != 0 || restored != 1 || retryCalls != 2 || retryReleased != 2 || unexpectedMute {
		t.Fatalf("retry did not restore and release only pending sessions: matched=%v err=%v", matched, err)
	}
}

func TestSilentAudioChild(t *testing.T) {
	if os.Getenv("LUNABOX_AUDIO_TEST_CHILD") != "1" {
		return
	}
	winmm := windows.NewLazySystemDLL("winmm.dll")
	var format [18]byte // WAVEFORMATEX: mono, 44.1 kHz, signed 16-bit PCM.
	binary.LittleEndian.PutUint16(format[0:], 1)
	binary.LittleEndian.PutUint16(format[2:], 1)
	binary.LittleEndian.PutUint32(format[4:], 44100)
	binary.LittleEndian.PutUint32(format[8:], 88200)
	binary.LittleEndian.PutUint16(format[12:], 2)
	binary.LittleEndian.PutUint16(format[14:], 16)
	var handle uintptr
	r, _, _ := winmm.NewProc("waveOutOpen").Call(uintptr(unsafe.Pointer(&handle)), uintptr(^uint32(0)), uintptr(unsafe.Pointer(&format[0])), 0, 0, 0)
	if r != 0 {
		t.Fatalf("waveOutOpen: %d", r)
	}
	defer winmm.NewProc("waveOutClose").Call(handle)
	silence := make([]byte, 88200)
	header := struct {
		data             uintptr
		length, recorded uint32
		user             uintptr
		flags, loops     uint32
		next, reserved   uintptr
	}{data: uintptr(unsafe.Pointer(&silence[0])), length: uint32(len(silence)), flags: 12, loops: 10000}
	r, _, _ = winmm.NewProc("waveOutPrepareHeader").Call(handle, uintptr(unsafe.Pointer(&header)), unsafe.Sizeof(header))
	if r != 0 {
		t.Fatalf("waveOutPrepareHeader: %d", r)
	}
	defer winmm.NewProc("waveOutUnprepareHeader").Call(handle, uintptr(unsafe.Pointer(&header)), unsafe.Sizeof(header))
	r, _, _ = winmm.NewProc("waveOutWrite").Call(handle, uintptr(unsafe.Pointer(&header)), unsafe.Sizeof(header))
	if r != 0 {
		t.Fatalf("waveOutWrite: %d", r)
	}
	fmt.Println("AUDIO_READY")
	bufio.NewReader(os.Stdin).ReadString('\n')
	winmm.NewProc("waveOutReset").Call(handle)
	runtime.KeepAlive(silence)
	runtime.KeepAlive(header)
}

func startSilentAudioChild(t *testing.T) (*exec.Cmd, io.WriteCloser) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSilentAudioChild$")
	cmd.Env = append(os.Environ(), "LUNABOX_AUDIO_TEST_CHILD=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			SetProcessMuted(uint32(cmd.Process.Pid), false)
			fmt.Fprintln(input, "stop")
			cmd.Wait()
		}
		input.Close()
	})
	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		if scanner.Text() == "AUDIO_READY" {
			return cmd, input
		}
	}
	t.Fatal("silent helper did not become ready")
	return nil, nil
}

// The callback runs while COM is initialized and owns no interface references.
func withTestAudioVolume(pid uint32, callback func(*iSimpleAudioVolume)) (bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, windows.COINIT_MULTITHREADED)
	if err := checkHRESULT(hr, "COM"); err != nil {
		return false, err
	}
	defer procCoUninitialize.Call()
	var enumerator *iMMDeviceEnumerator
	hr, _, _ = procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)), 0, clsctxAll, uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)), uintptr(unsafe.Pointer(&enumerator)))
	if err := checkHRESULT(hr, "enumerator"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(enumerator))
	var device *iMMDevice
	hr, _, _ = syscall.SyscallN(enumerator.vtbl.getDefaultAudioEndpoint, uintptr(unsafe.Pointer(enumerator)), eRender, eMultimedia, uintptr(unsafe.Pointer(&device)))
	if err := checkHRESULT(hr, "device"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(device))
	var manager *iAudioSessionManager2
	hr, _, _ = syscall.SyscallN(device.vtbl.activate, uintptr(unsafe.Pointer(device)), uintptr(unsafe.Pointer(&iidIAudioSessionManager2)), clsctxAll, 0, uintptr(unsafe.Pointer(&manager)))
	if err := checkHRESULT(hr, "manager"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(manager))
	var sessions *iAudioSessionEnumerator
	hr, _, _ = syscall.SyscallN(manager.vtbl.getSessionEnumerator, uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(&sessions)))
	if err := checkHRESULT(hr, "sessions"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessions))
	var count int32
	hr, _, _ = syscall.SyscallN(sessions.vtbl.getCount, uintptr(unsafe.Pointer(sessions)), uintptr(unsafe.Pointer(&count)))
	if err := checkHRESULT(hr, "count"); err != nil {
		return false, err
	}
	for index := int32(0); index < count; index++ {
		var control *iUnknown
		hr, _, _ = syscall.SyscallN(sessions.vtbl.getSession, uintptr(unsafe.Pointer(sessions)), uintptr(index), uintptr(unsafe.Pointer(&control)))
		if err := checkHRESULT(hr, "session"); err != nil {
			return false, err
		}
		defer release(unsafe.Pointer(control))
		var control2 *iAudioSessionControl2
		if err := queryInterface(unsafe.Pointer(control), &iidIAudioSessionControl2, unsafe.Pointer(&control2)); err != nil {
			continue
		}
		defer release(unsafe.Pointer(control2))
		var sessionPID uint32
		hr, _, _ = syscall.SyscallN(control2.vtbl.getProcessID, uintptr(unsafe.Pointer(control2)), uintptr(unsafe.Pointer(&sessionPID)))
		if err := checkHRESULT(hr, "pid"); err != nil {
			return false, err
		}
		if sessionPID != pid {
			continue
		}
		var volume *iSimpleAudioVolume
		if err := queryInterface(unsafe.Pointer(control), &iidISimpleAudioVolume, unsafe.Pointer(&volume)); err != nil {
			return false, err
		}
		defer release(unsafe.Pointer(volume))
		callback(volume)
		return true, nil
	}
	return false, nil
}

func waitForTestAudio(t *testing.T, pid uint32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		found, err := withTestAudioVolume(pid, func(*iSimpleAudioVolume) {})
		if err != nil {
			t.Fatal(err)
		}
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no helper audio session")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func testAudioMuted(t *testing.T, pid uint32) bool {
	t.Helper()
	var muted int32
	found, err := withTestAudioVolume(pid, func(volume *iSimpleAudioVolume) {
		hr, _, _ := syscall.SyscallN(volume.vtbl.getMute, uintptr(unsafe.Pointer(volume)), uintptr(unsafe.Pointer(&muted)))
		if err := checkHRESULT(hr, "get mute"); err != nil {
			t.Fatal(err)
		}
	})
	if err != nil || !found {
		t.Fatalf("read helper mute: found=%v err=%v", found, err)
	}
	return muted != 0
}
