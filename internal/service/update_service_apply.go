package service

import (
	"fmt"
	"time"

	"lunabox/internal/updateclient"
	"lunabox/internal/version"
)

type UpdateProgress struct {
	Phase      string `json:"phase"`
	File       string `json:"file,omitempty"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Fallback   bool   `json:"fallback"`
}

type UpdateApplyResult struct {
	Started      bool `json:"started"`
	FallbackUsed bool `json:"fallback_used"`
	FileCount    int  `json:"file_count"`
}

// DownloadAndApplyUpdate downloads verified update artifacts with LunaBox's
// existing downloader, asks the standalone updater to prepare them, then starts
// the updater in commit mode and enters the normal LunaBox shutdown flow.
func (s *UpdateService) DownloadAndApplyUpdate(manifestURL string) (*UpdateApplyResult, error) {
	if !s.applyMu.TryLock() {
		return nil, fmt.Errorf("an update is already in progress")
	}
	defer s.applyMu.Unlock()

	if s.ctx == nil || s.config == nil {
		return nil, fmt.Errorf("update service is not initialized")
	}
	if s.quitHandler == nil {
		return nil, fmt.Errorf("update shutdown handler is not configured")
	}
	appConfig, err := s.config.GetAppConfig()
	if err != nil {
		return nil, fmt.Errorf("get update configuration: %w", err)
	}

	result, err := updateclient.Apply(s.ctx, updateclient.Options{
		ManifestURL:     manifestURL,
		CurrentVersion:  version.Version,
		BuildMode:       version.BuildMode,
		UserAgent:       version.UserAgent(),
		Config:          &appConfig,
		CompareVersions: compareVersions,
		Progress: func(progress updateclient.Progress) {
			s.runtime.Emit("update:progress", UpdateProgress{
				Phase:      progress.Phase,
				File:       progress.File,
				Downloaded: progress.Downloaded,
				Total:      progress.Total,
				Percent:    progress.Percent,
				Fallback:   progress.Fallback,
			})
		},
	})
	if err != nil {
		return nil, err
	}
	response := &UpdateApplyResult{
		Started:      result.Started,
		FallbackUsed: result.FallbackUsed,
		FileCount:    result.FileCount,
	}
	if result.Started {
		go func() {
			time.Sleep(150 * time.Millisecond)
			s.quitHandler()
		}()
	}
	return response, nil
}
