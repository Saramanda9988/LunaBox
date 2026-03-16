package processutils

// ProcessInfo 进程信息。
type ProcessInfo struct {
	Name string `json:"name"`
	PID  uint32 `json:"pid"`
}
