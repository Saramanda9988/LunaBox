//go:build windows

package audioutils

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	clsctxAll         = 0x17
	eRender           = 0
	eMultimedia       = 1
	deviceStateActive = 1
)

var (
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx       = ole32.NewProc("CoInitializeEx")
	procCoUninitialize       = ole32.NewProc("CoUninitialize")
	procCoCreateInstance     = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	clsidMMDeviceEnumerator  = windows.GUID{Data1: 0xbcde0395, Data2: 0xe52f, Data3: 0x467c, Data4: [8]byte{0x8e, 0x3d, 0xc4, 0x57, 0x92, 0x91, 0x69, 0x2e}}
	iidIMMDeviceEnumerator   = windows.GUID{Data1: 0xa95664d2, Data2: 0x9614, Data3: 0x4f35, Data4: [8]byte{0xa7, 0x46, 0xde, 0x8d, 0xb6, 0x36, 0x17, 0xe6}}
	iidIAudioSessionManager2 = windows.GUID{Data1: 0x77aa99a0, Data2: 0x1bd6, Data3: 0x484f, Data4: [8]byte{0x8b, 0xc7, 0x2c, 0x65, 0x4c, 0x9a, 0x9b, 0x6f}}
	iidIAudioSessionControl2 = windows.GUID{Data1: 0xbfb7ff88, Data2: 0x7239, Data3: 0x4fc9, Data4: [8]byte{0x8f, 0xa2, 0x07, 0xc9, 0x50, 0xbe, 0x9c, 0x6d}}
	iidISimpleAudioVolume    = windows.GUID{Data1: 0x87ce5498, Data2: 0x68d6, Data3: 0x44e5, Data4: [8]byte{0x92, 0x15, 0x6d, 0xa4, 0x7e, 0xf8, 0x83, 0xd8}}
)

type iUnknown struct {
	vtbl *iUnknownVtbl
}

type iUnknownVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
}

type iMMDeviceEnumerator struct {
	vtbl *iMMDeviceEnumeratorVtbl
}

type iMMDeviceEnumeratorVtbl struct {
	iUnknownVtbl
	enumAudioEndpoints                     uintptr
	getDefaultAudioEndpoint                uintptr
	getDevice                              uintptr
	registerEndpointNotificationCallback   uintptr
	unregisterEndpointNotificationCallback uintptr
}

type iMMDevice struct {
	vtbl *iMMDeviceVtbl
}

type iMMDeviceCollection struct {
	vtbl *iMMDeviceCollectionVtbl
}

type iMMDeviceCollectionVtbl struct {
	iUnknownVtbl
	getCount uintptr
	item     uintptr
}

type iMMDeviceVtbl struct {
	iUnknownVtbl
	activate          uintptr
	openPropertyStore uintptr
	getID             uintptr
	getState          uintptr
}

type iAudioSessionManager2 struct {
	vtbl *iAudioSessionManager2Vtbl
}

type iAudioSessionManager2Vtbl struct {
	iUnknownVtbl
	getAudioSessionControl        uintptr
	getSimpleAudioVolume          uintptr
	getSessionEnumerator          uintptr
	registerSessionNotification   uintptr
	unregisterSessionNotification uintptr
	registerDuckNotification      uintptr
	unregisterDuckNotification    uintptr
}

type iAudioSessionEnumerator struct {
	vtbl *iAudioSessionEnumeratorVtbl
}

type iAudioSessionEnumeratorVtbl struct {
	iUnknownVtbl
	getCount   uintptr
	getSession uintptr
}

type iAudioSessionControl2 struct {
	vtbl *iAudioSessionControl2Vtbl
}

type iAudioSessionControl2Vtbl struct {
	iUnknownVtbl
	getState                           uintptr
	getDisplayName                     uintptr
	setDisplayName                     uintptr
	getIconPath                        uintptr
	setIconPath                        uintptr
	getGroupingParam                   uintptr
	setGroupingParam                   uintptr
	registerAudioSessionNotification   uintptr
	unregisterAudioSessionNotification uintptr
	getSessionIdentifier               uintptr
	getSessionInstanceIdentifier       uintptr
	getProcessID                       uintptr
	isSystemSoundsSession              uintptr
	setDuckingPreference               uintptr
}

type iSimpleAudioVolume struct {
	vtbl *iSimpleAudioVolumeVtbl
}

type iSimpleAudioVolumeVtbl struct {
	iUnknownVtbl
	setMasterVolume uintptr
	getMasterVolume uintptr
	setMute         uintptr
	getMute         uintptr
}

func IsProcessMuteSupported() bool {
	return true
}

type retainedAudioSession struct {
	control *iAudioSessionControl2
	volume  *iSimpleAudioVolume
}

type processMuteState struct {
	sessions map[uint32]map[string]retainedAudioSession
}

type processMuteResult struct {
	matched bool
	err     error
}

type processMuteRequest struct {
	processID uint32
	muted     bool
	result    chan processMuteResult
}

// All retained COM interfaces belong to this worker's MTA. Keeping the thread
// initialized also keeps the apartment alive between focus updates.
var processMuteRequests = sync.OnceValue(func() chan processMuteRequest {
	requests := make(chan processMuteRequest)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		state := processMuteState{sessions: make(map[uint32]map[string]retainedAudioSession)}
		initialized := false
		defer func() {
			if initialized {
				procCoUninitialize.Call()
			}
		}()
		for request := range requests {
			if !initialized {
				hr, _, _ := procCoInitializeEx.Call(0, windows.COINIT_MULTITHREADED)
				if err := checkHRESULT(hr, "initialize COM"); err != nil {
					request.result <- processMuteResult{err: err}
					continue
				}
				initialized = true
			}
			matched, err := state.setProcessMuted(request.processID, request.muted)
			request.result <- processMuteResult{matched: matched, err: err}
		}
	}()
	return requests
})

// SetProcessMuted retains every session it mutes until restoration succeeds.
// The retained controls remain usable after the game's streams and PID exit;
// enumerating that PID again would miss the expired sessions and leave Windows'
// persistent mute setting behind. matched includes partial success on errors.
func SetProcessMuted(processID uint32, muted bool) (matched bool, err error) {
	if processID == 0 {
		return false, nil
	}
	result := make(chan processMuteResult, 1)
	processMuteRequests() <- processMuteRequest{processID: processID, muted: muted, result: result}
	response := <-result
	return response.matched, response.err
}

func (s *processMuteState) restoreRetainedSessions(processID uint32) (matched bool, err error) {
	for id, session := range s.sessions[processID] {
		if restoreErr := setAudioVolumeMuted(session.volume, false); restoreErr != nil {
			if err == nil {
				err = restoreErr
			}
			continue // Keep failed references for the next restoration attempt.
		}
		matched = true
		release(unsafe.Pointer(session.volume))
		release(unsafe.Pointer(session.control))
		delete(s.sessions[processID], id)
	}
	if len(s.sessions[processID]) == 0 {
		delete(s.sessions, processID)
	}
	return matched, err
}

func (s *processMuteState) setProcessMuted(processID uint32, muted bool) (matched bool, err error) {
	if !muted && len(s.sessions[processID]) > 0 {
		// Restore only the sessions we changed, even if the default device changed.
		return s.restoreRetainedSessions(processID)
	}

	var deviceEnumerator *iMMDeviceEnumerator
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidIMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&deviceEnumerator)),
	)
	if err := checkHRESULT(hr, "create audio device enumerator"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(deviceEnumerator))
	if !muted {
		// A fresh foreground process may have inherited mute from an older build.
		return s.restoreProcessOnActiveDevices(deviceEnumerator, processID)
	}

	var device *iMMDevice
	hr, _, _ = syscall.SyscallN(
		deviceEnumerator.vtbl.getDefaultAudioEndpoint,
		uintptr(unsafe.Pointer(deviceEnumerator)),
		eRender,
		eMultimedia,
		uintptr(unsafe.Pointer(&device)),
	)
	if err := checkHRESULT(hr, "get default audio endpoint"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(device))
	return s.setDeviceProcessMuted(device, processID, muted)
}

func (s *processMuteState) restoreProcessOnActiveDevices(enumerator *iMMDeviceEnumerator, processID uint32) (matched bool, err error) {
	var devices *iMMDeviceCollection
	hr, _, _ := syscall.SyscallN(
		enumerator.vtbl.enumAudioEndpoints,
		uintptr(unsafe.Pointer(enumerator)),
		eRender,
		deviceStateActive,
		uintptr(unsafe.Pointer(&devices)),
	)
	if err := checkHRESULT(hr, "enumerate output devices for audio restoration"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(devices))
	var count uint32
	hr, _, _ = syscall.SyscallN(devices.vtbl.getCount, uintptr(unsafe.Pointer(devices)), uintptr(unsafe.Pointer(&count)))
	if err := checkHRESULT(hr, "count output devices"); err != nil {
		return false, err
	}
	var firstErr error
	for index := uint32(0); index < count; index++ {
		var device *iMMDevice
		hr, _, _ = syscall.SyscallN(devices.vtbl.item, uintptr(unsafe.Pointer(devices)), uintptr(index), uintptr(unsafe.Pointer(&device)))
		deviceErr := checkHRESULT(hr, "get output device")
		if deviceErr == nil {
			var changed bool
			changed, deviceErr = s.setDeviceProcessMuted(device, processID, false)
			matched = matched || changed
			release(unsafe.Pointer(device))
		}
		if deviceErr != nil && firstErr == nil {
			firstErr = deviceErr
		}
	}
	return matched, firstErr
}

func (s *processMuteState) setDeviceProcessMuted(device *iMMDevice, processID uint32, muted bool) (matched bool, err error) {
	var sessionManager *iAudioSessionManager2
	hr, _, _ := syscall.SyscallN(
		device.vtbl.activate,
		uintptr(unsafe.Pointer(device)),
		uintptr(unsafe.Pointer(&iidIAudioSessionManager2)),
		clsctxAll,
		0,
		uintptr(unsafe.Pointer(&sessionManager)),
	)
	if err := checkHRESULT(hr, "activate audio session manager"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionManager))

	var sessionEnumerator *iAudioSessionEnumerator
	hr, _, _ = syscall.SyscallN(
		sessionManager.vtbl.getSessionEnumerator,
		uintptr(unsafe.Pointer(sessionManager)),
		uintptr(unsafe.Pointer(&sessionEnumerator)),
	)
	if err := checkHRESULT(hr, "enumerate audio sessions"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionEnumerator))

	var count int32
	hr, _, _ = syscall.SyscallN(
		sessionEnumerator.vtbl.getCount,
		uintptr(unsafe.Pointer(sessionEnumerator)),
		uintptr(unsafe.Pointer(&count)),
	)
	if err := checkHRESULT(hr, "count audio sessions"); err != nil {
		return false, err
	}

	var firstErr error
	for index := int32(0); index < count; index++ {
		changed, sessionErr := s.setSessionMuted(sessionEnumerator, index, processID, muted)
		if changed {
			matched = true
		}
		if sessionErr != nil && firstErr == nil {
			firstErr = sessionErr
		}
	}
	return matched, firstErr
}

func (s *processMuteState) setSessionMuted(enumerator *iAudioSessionEnumerator, index int32, processID uint32, muted bool) (bool, error) {
	var sessionControl *iUnknown
	hr, _, _ := syscall.SyscallN(
		enumerator.vtbl.getSession,
		uintptr(unsafe.Pointer(enumerator)),
		uintptr(index),
		uintptr(unsafe.Pointer(&sessionControl)),
	)
	if err := checkHRESULT(hr, "get audio session"); err != nil {
		return false, err
	}
	defer release(unsafe.Pointer(sessionControl))

	var sessionControl2 *iAudioSessionControl2
	if err := queryInterface(unsafe.Pointer(sessionControl), &iidIAudioSessionControl2, unsafe.Pointer(&sessionControl2)); err != nil {
		return false, nil
	}
	retained := false
	defer func() {
		if !retained {
			release(unsafe.Pointer(sessionControl2))
		}
	}()

	var sessionProcessID uint32
	hr, _, _ = syscall.SyscallN(
		sessionControl2.vtbl.getProcessID,
		uintptr(unsafe.Pointer(sessionControl2)),
		uintptr(unsafe.Pointer(&sessionProcessID)),
	)
	if err := checkHRESULT(hr, "get audio session process ID"); err != nil {
		return false, err
	}
	if sessionProcessID != processID {
		return false, nil
	}

	var volume *iSimpleAudioVolume
	if err := queryInterface(unsafe.Pointer(sessionControl), &iidISimpleAudioVolume, unsafe.Pointer(&volume)); err != nil {
		return false, err
	}
	defer func() {
		if !retained {
			release(unsafe.Pointer(volume))
		}
	}()

	var sessionID string
	if muted {
		var id *uint16
		hr, _, _ = syscall.SyscallN(sessionControl2.vtbl.getSessionInstanceIdentifier, uintptr(unsafe.Pointer(sessionControl2)), uintptr(unsafe.Pointer(&id)))
		if err := checkHRESULT(hr, "get audio session instance ID"); err != nil {
			return false, err // Never mute a session that cannot be retained.
		}
		sessionID = windows.UTF16PtrToString(id)
		procCoTaskMemFree.Call(uintptr(unsafe.Pointer(id)))
	}
	if err := setAudioVolumeMuted(volume, muted); err != nil {
		return false, err
	}
	if muted {
		if s.sessions[processID] == nil {
			s.sessions[processID] = make(map[string]retainedAudioSession)
		}
		if _, exists := s.sessions[processID][sessionID]; !exists {
			s.sessions[processID][sessionID] = retainedAudioSession{control: sessionControl2, volume: volume}
			retained = true
		}
	}
	return true, nil
}

func setAudioVolumeMuted(volume *iSimpleAudioVolume, muted bool) error {
	muteValue := uintptr(0)
	if muted {
		muteValue = 1
	}
	eventContext := windows.GUID{}
	hr, _, _ := syscall.SyscallN(
		volume.vtbl.setMute,
		uintptr(unsafe.Pointer(volume)),
		muteValue,
		uintptr(unsafe.Pointer(&eventContext)),
	)
	return checkHRESULT(hr, "set audio session mute state")
}

func queryInterface(instance unsafe.Pointer, iid *windows.GUID, destination unsafe.Pointer) error {
	unknown := (*iUnknown)(instance)
	hr, _, _ := syscall.SyscallN(
		unknown.vtbl.queryInterface,
		uintptr(instance),
		uintptr(unsafe.Pointer(iid)),
		uintptr(destination),
	)
	return checkHRESULT(hr, "query audio interface")
}

func release(instance unsafe.Pointer) {
	if instance == nil {
		return
	}
	unknown := (*iUnknown)(instance)
	syscall.SyscallN(unknown.vtbl.release, uintptr(instance))
}

func checkHRESULT(hr uintptr, operation string) error {
	if int32(hr) >= 0 {
		return nil
	}
	return fmt.Errorf("%s: HRESULT 0x%08X", operation, uint32(hr))
}
