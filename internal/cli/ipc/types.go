package ipc

import (
	"time"
)

const (
	ServerAddr  = "127.0.0.1"
	Port        = 56789
	ServerURL   = "http://127.0.0.1:56789"
	PingTimeout = 500 * time.Millisecond
)

type CommandRequest struct {
	Args []string `json:"args"`
}

type CommandResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// InstallResponse IPC /install 响应
type InstallResponse struct {
	TaskID string `json:"task_id,omitempty"`
	Error  string `json:"error,omitempty"`
}
