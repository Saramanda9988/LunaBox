//go:build !windows

package protocol

import "fmt"

func RegisterURLScheme(_ string) error {
	return fmt.Errorf("register-protocol is only supported on Windows")
}

func UnregisterURLScheme() error {
	return fmt.Errorf("unregister-protocol is only supported on Windows")
}
