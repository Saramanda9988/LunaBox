//go:build !linux

package service

import launcherpkg "lunabox/internal/service/launcher"

func (s *StartService) startExitWatch(session *activePlaySession, processID uint32, processName string, exitWatch launcherpkg.ExitWatch) (<-chan struct{}, bool) {
	return nil, false
}
