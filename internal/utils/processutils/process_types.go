package processutils

// ProcessInfo 进程信息。
type ProcessInfo struct {
	Name string `json:"name"`
	PID  uint32 `json:"pid"`
}

// ProcessCommandInfo contains the command arguments and environment captured
// for a running process.
type ProcessCommandInfo struct {
	ExecutablePath string
	Arguments      []string
	Environment    map[string]string
}

// StartedProcess describes a process started through a Windows API that returns
// an owned process handle. The caller must either pass Handle to a ProcessMonitor
// or close it with CloseProcessHandle.
type StartedProcess struct {
	PID      uint32
	Handle   uintptr
	ExitChan <-chan struct{}
}
