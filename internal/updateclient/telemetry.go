package updateclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/updater/updateutils"

	"github.com/google/uuid"
)

const pendingUpdateFileName = "pending-update.json"

type telemetryEvent struct {
	EventID          string `json:"event_id"`
	TransactionID    string `json:"transaction_id,omitempty"`
	EventType        string `json:"event_type"`
	CurrentVersion   string `json:"current_version,omitempty"`
	TargetVersion    string `json:"target_version"`
	Channel          string `json:"channel"`
	Architecture     string `json:"architecture"`
	BuildMode        string `json:"build_mode"`
	Artifact         string `json:"artifact,omitempty"`
	TransferredBytes int64  `json:"transferred_bytes,omitempty"`
	FailureCode      string `json:"failure_code,omitempty"`
	ClientTime       string `json:"client_time"`
}

type pendingUpdate struct {
	EventID        string `json:"event_id"`
	EventURL       string `json:"event_url"`
	WorkDir        string `json:"work_dir"`
	TransactionID  string `json:"transaction_id"`
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version"`
	Channel        string `json:"channel"`
	BuildMode      string `json:"build_mode"`
}

func newTelemetryEvent(eventType string, transactionID string, currentVersion string, targetVersion string, channel string, buildMode string) telemetryEvent {
	return telemetryEvent{
		EventID:        uuid.NewString(),
		TransactionID:  transactionID,
		EventType:      eventType,
		CurrentVersion: currentVersion,
		TargetVersion:  targetVersion,
		Channel:        channel,
		Architecture:   runtime.GOARCH,
		BuildMode:      buildMode,
		ClientTime:     time.Now().UTC().Format(time.RFC3339),
	}
}

func reportEvent(ctx context.Context, config *appconf.AppConfig, userAgent string, eventURL string, event telemetryEvent) error {
	if strings.TrimSpace(eventURL) == "" {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	client, _, err := downloadutils.NewSecureHTTPClientFromConfig(5*time.Second, config)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, eventURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", userAgent)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("update event endpoint returned status %d", response.StatusCode)
	}
	return nil
}

func writePendingUpdate(state pendingUpdate) error {
	statePath, err := pendingUpdatePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tempPath, statePath); err != nil {
		if removeErr := os.Remove(statePath); removeErr != nil && !os.IsNotExist(removeErr) {
			_ = os.Remove(tempPath)
			return err
		}
		if retryErr := os.Rename(tempPath, statePath); retryErr != nil {
			_ = os.Remove(tempPath)
			return retryErr
		}
	}
	return nil
}

func removePendingUpdate() {
	statePath, err := pendingUpdatePath()
	if err == nil {
		_ = os.Remove(statePath)
	}
}

// ReportPendingResult reports the updater result after the new LunaBox process
// starts. Failed sends retain the pending marker for the next launch.
func ReportPendingResult(ctx context.Context, config *appconf.AppConfig, userAgent string) error {
	statePath, err := pendingUpdatePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var state pendingUpdate
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode pending update: %w", err)
	}
	if !isUpdateWorkDir(state.WorkDir) {
		return fmt.Errorf("pending update work directory is invalid")
	}
	resultData, err := os.ReadFile(filepath.Join(state.WorkDir, "result.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var result updateutils.UpdateResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return fmt.Errorf("decode update result: %w", err)
	}
	if result.TransactionID != state.TransactionID || result.TargetVersion != state.TargetVersion {
		return fmt.Errorf("update result does not match pending transaction")
	}

	eventType := "install_success"
	failureCode := ""
	if !result.Success {
		eventType = "install_failed"
		failureCode = "updater_failed"
	}
	event := newTelemetryEvent(eventType, state.TransactionID, state.CurrentVersion, state.TargetVersion, state.Channel, state.BuildMode)
	event.EventID = state.EventID
	event.FailureCode = failureCode
	if err := reportEvent(ctx, config, userAgent, state.EventURL, event); err != nil {
		return err
	}
	removePendingUpdate()
	return nil
}

func pendingUpdatePath() (string, error) {
	dir, err := apputils.GetCacheSubDir("updates")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pendingUpdateFileName), nil
}

func isUpdateWorkDir(workDir string) bool {
	workDir, err := filepath.Abs(workDir)
	if err != nil {
		return false
	}
	tempDir, err := filepath.Abs(os.TempDir())
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(tempDir, workDir)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	return strings.HasPrefix(filepath.Base(workDir), "LunaBox-update-")
}
