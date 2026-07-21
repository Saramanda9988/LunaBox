//go:build windows

package updateutils

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const replaceFileWriteThrough = 0x00000001

var replaceFileW = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func waitForProcessExit(pid int, timeout time.Duration) error {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(handle)

	milliseconds := uint32(timeout / time.Millisecond)
	if timeout <= 0 || milliseconds == 0 {
		milliseconds = windows.INFINITE
	}
	result, err := windows.WaitForSingleObject(handle, milliseconds)
	if err != nil {
		return err
	}
	if result == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return nil
}

func replaceFile(targetPath string, replacementPath string, backupPath string) (bool, error) {
	_, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return false, os.Rename(replacementPath, targetPath)
	}
	if err != nil {
		return false, err
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return true, err
	}
	replacement, err := windows.UTF16PtrFromString(replacementPath)
	if err != nil {
		return true, err
	}
	backup, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return true, err
	}
	if err := callReplaceFile(target, replacement, backup); err != nil {
		return true, err
	}
	return true, nil
}

func restoreBackup(targetPath string, backupPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return err
	}
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return os.Rename(backupPath, targetPath)
	}
	target, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}
	backup, err := windows.UTF16PtrFromString(backupPath)
	if err != nil {
		return err
	}
	return callReplaceFile(target, backup, nil)
}

func callReplaceFile(target *uint16, replacement *uint16, backup *uint16) error {
	result, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(target)),
		uintptr(unsafe.Pointer(replacement)),
		uintptr(unsafe.Pointer(backup)),
		replaceFileWriteThrough,
		0,
		0,
	)
	if result == 0 {
		return callErr
	}
	return nil
}

func updateInstallMetadata(buildMode string, version string) error {
	if buildMode != "installer" {
		return nil
	}
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\LunaBoxLunaBox`,
		registry.SET_VALUE,
	)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue("DisplayVersion", version)
}

func configureRestartCommand(command *exec.Cmd) error {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	return nil
}
