package service

import "sync"

const startupFailedEvent = "startup:failed"

// StartupFailure contains the diagnostic message shown when LunaBox cannot
// create its main window.
type StartupFailure struct {
	Message string `json:"message"`
}

// StartupService keeps the latest startup failure available in case the
// hidden error window loads after the failure event was emitted.
type StartupService struct {
	mu        sync.RWMutex
	failure   StartupFailure
	emitEvent func(string, ...interface{})
}

func NewStartupService() *StartupService {
	return &StartupService{
		emitEvent: func(string, ...interface{}) {},
	}
}

func (s *StartupService) GetFailure() StartupFailure {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failure
}

// SetEventEmitter injects the application event bus.
//
//wails:ignore
func (s *StartupService) SetEventEmitter(emit func(string, ...interface{})) {
	if emit == nil {
		return
	}

	s.mu.Lock()
	s.emitEvent = emit
	s.mu.Unlock()
}

// ReportFailure stores and broadcasts a startup error to the hidden error
// window before that window becomes visible.
//
//wails:ignore
func (s *StartupService) ReportFailure(message string) {
	failure := StartupFailure{Message: message}
	s.mu.Lock()
	s.failure = failure
	emit := s.emitEvent
	s.mu.Unlock()

	emit(startupFailedEvent, failure)
}
