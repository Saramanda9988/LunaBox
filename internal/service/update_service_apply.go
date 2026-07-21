package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/processutils"
	"lunabox/internal/utils/updateutils"
	"lunabox/internal/version"

	"github.com/google/uuid"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateManifestMaxBytes = 4 * 1024 * 1024
	updaterExecutableName  = "LunaBoxUpdater.exe"
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

type selectedUpdateFile struct {
	release updateutils.ReleaseFile
	task    updateutils.TaskFile
}

// DownloadAndApplyUpdate downloads verified update artifacts with LunaBox's
// existing downloader, asks the standalone updater to prepare them, then starts
// the updater in commit mode and enters the normal LunaBox shutdown flow.
func (s *UpdateService) DownloadAndApplyUpdate(manifestURL string) (*UpdateApplyResult, error) {
	if !s.applyMu.TryLock() {
		return nil, fmt.Errorf("an update is already in progress")
	}
	defer s.applyMu.Unlock()

	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("in-app updates are currently supported on Windows only")
	}
	if s.ctx == nil || s.config == nil {
		return nil, fmt.Errorf("update service is not initialized")
	}
	if s.quitHandler == nil {
		return nil, fmt.Errorf("update shutdown handler is not configured")
	}
	if err := downloadutils.ValidateDownloadURL(manifestURL); err != nil {
		return nil, fmt.Errorf("invalid update manifest url: %w", err)
	}

	manifest, channel, err := s.fetchReleaseManifest(manifestURL)
	if err != nil {
		return nil, err
	}
	hasUpdate, err := compareVersions(version.Version, manifest.Version)
	if err != nil {
		return nil, fmt.Errorf("compare update manifest version: %w", err)
	}
	if !hasUpdate {
		return nil, fmt.Errorf("update manifest version %s is not newer than %s", manifest.Version, version.Version)
	}

	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve LunaBox executable: %w", err)
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute LunaBox executable: %w", err)
	}
	appDir := filepath.Dir(executablePath)
	installedUpdater := filepath.Join(appDir, updaterExecutableName)
	if info, statErr := os.Stat(installedUpdater); statErr != nil || info.IsDir() {
		return nil, fmt.Errorf("%s is missing; download the full release for this update", updaterExecutableName)
	}

	workDir, err := os.MkdirTemp("", "LunaBox-update-"+safeUpdatePathPart(manifest.Version)+"-")
	if err != nil {
		return nil, fmt.Errorf("create update transaction: %w", err)
	}
	transactionID := uuid.NewString()
	runnerPath := filepath.Join(workDir, "runner", updaterExecutableName)
	if err := apputils.CopyFile(installedUpdater, runnerPath); err != nil {
		return nil, fmt.Errorf("copy updater to transaction directory: %w", err)
	}

	selected, err := selectUpdateFiles(channel, appDir, version.Version, workDir)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return &UpdateApplyResult{Started: false}, nil
	}

	downloader, _, err := downloadutils.NewDownloader(downloadutils.TransferConfig{
		ProxyConfig: s.config.config,
		UserAgent:   "LunaBox-Updater/1.0",
	})
	if err != nil {
		return nil, fmt.Errorf("create update downloader: %w", err)
	}

	task := &updateutils.Task{
		SchemaVersion: updateutils.TaskSchemaVersion,
		TransactionID: transactionID,
		TargetVersion: manifest.Version,
		BuildMode:     version.BuildMode,
		AppDir:        appDir,
		WorkDir:       workDir,
		WaitPID:       os.Getpid(),
		WaitTimeout:   600,
		RestartPath:   "LunaBox.exe",
		Files:         make([]updateutils.TaskFile, 0, len(selected)),
	}

	totalBytes := selectedArtifactTotal(selected)
	var completedBytes int64
	for i := range selected {
		item := &selected[i]
		artifact := artifactForTask(item.release, item.task.Kind)
		if err := s.downloadUpdateArtifact(downloader, artifact, item.task.ArtifactPath, item.task.Path, completedBytes, totalBytes, false); err != nil {
			return nil, err
		}
		completedBytes += artifact.Size
		task.Files = append(task.Files, item.task)
	}

	taskPath := filepath.Join(workDir, "task.json")
	if err := updateutils.WriteTask(taskPath, task); err != nil {
		return nil, fmt.Errorf("write updater task: %w", err)
	}
	s.emitUpdateProgress(UpdateProgress{Phase: "preparing", Total: totalBytes, Downloaded: totalBytes, Percent: 100})
	prepareErr := runUpdaterPrepare(runnerPath, taskPath, workDir)
	fallbackUsed := false
	if prepareErr != nil && taskUsesPatch(task) {
		fallbackUsed = true
		s.emitUpdateProgress(UpdateProgress{Phase: "fallback", Fallback: true})
		if err := s.replacePatchesWithFullDownloads(downloader, selected, task); err != nil {
			return nil, fmt.Errorf("patch prepare failed (%v), and full fallback failed: %w", prepareErr, err)
		}
		if err := updateutils.WriteTask(taskPath, task); err != nil {
			return nil, fmt.Errorf("write full fallback task: %w", err)
		}
		prepareErr = runUpdaterPrepare(runnerPath, taskPath, workDir)
	}
	if prepareErr != nil {
		return nil, fmt.Errorf("prepare update: %w", prepareErr)
	}

	s.emitUpdateProgress(UpdateProgress{Phase: "ready", Percent: 100, Fallback: fallbackUsed})
	if err := startUpdaterCommit(runnerPath, taskPath, workDir, version.BuildMode == "installer"); err != nil {
		return nil, err
	}

	result := &UpdateApplyResult{
		Started:      true,
		FallbackUsed: fallbackUsed,
		FileCount:    len(task.Files),
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		s.quitHandler()
	}()
	return result, nil
}

func (s *UpdateService) fetchReleaseManifest(manifestURL string) (*updateutils.ReleaseManifest, updateutils.ReleaseChannel, error) {
	appConfig, err := s.config.GetAppConfig()
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("get proxy configuration: %w", err)
	}
	client, _, err := downloadutils.NewSecureHTTPClientFromConfig(20*time.Second, &appConfig)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("create update manifest client: %w", err)
	}
	request, err := http.NewRequestWithContext(s.ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, err
	}
	request.Header.Set("User-Agent", "LunaBox-Updater/1.0")
	response, err := client.Do(request)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("download update manifest: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("download update manifest: unexpected status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, updateManifestMaxBytes+1))
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("read update manifest: %w", err)
	}
	if len(data) > updateManifestMaxBytes {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("update manifest is too large")
	}
	var manifest updateutils.ReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("decode update manifest: %w", err)
	}
	channelName := fmt.Sprintf("windows-%s-%s", runtime.GOARCH, version.BuildMode)
	channel, err := manifest.Validate(channelName)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, err
	}
	return &manifest, channel, nil
}

func selectUpdateFiles(channel updateutils.ReleaseChannel, appDir string, currentVersion string, workDir string) ([]selectedUpdateFile, error) {
	selected := make([]selectedUpdateFile, 0, len(channel.Files))
	for _, releaseFile := range channel.Files {
		targetPath := filepath.Join(appDir, filepath.FromSlash(releaseFile.Path))
		currentSHA := ""
		_, statErr := os.Stat(targetPath)
		present := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspect installed %s: %w", releaseFile.Path, statErr)
		}
		if !present && releaseFile.InstallPolicy == updateutils.InstallPolicyIfPresent {
			continue
		}
		if present {
			sha, _, err := updateutils.FileSHA256(targetPath)
			if err != nil {
				return nil, fmt.Errorf("hash installed %s: %w", releaseFile.Path, err)
			}
			currentSHA = sha
			if strings.EqualFold(currentSHA, releaseFile.TargetSHA256) {
				continue
			}
		}

		kind := updateutils.TaskFileKindFull
		artifact := releaseFile.Full
		sourceSHA := ""
		if present && releaseFile.Patch != nil &&
			strings.EqualFold(currentSHA, releaseFile.Patch.SourceSHA256) &&
			versionsEqual(currentVersion, releaseFile.Patch.SourceVersion) {
			kind = updateutils.TaskFileKindPatch
			artifact = releaseFile.Patch.Artifact
			sourceSHA = releaseFile.Patch.SourceSHA256
		}
		artifactPath := filepath.Join(workDir, "artifacts", artifactFileName(releaseFile.Path, kind, artifact.Compression))
		selected = append(selected, selectedUpdateFile{
			release: releaseFile,
			task: updateutils.TaskFile{
				Path:           releaseFile.Path,
				Kind:           kind,
				ArtifactPath:   artifactPath,
				ArtifactSize:   artifact.Size,
				ArtifactSHA256: artifact.SHA256,
				Compression:    artifact.Compression,
				SourceSHA256:   sourceSHA,
				TargetSHA256:   releaseFile.TargetSHA256,
				TargetSize:     releaseFile.TargetSize,
			},
		})
	}
	return selected, nil
}

func (s *UpdateService) downloadUpdateArtifact(
	downloader *downloadutils.Downloader,
	artifact updateutils.Artifact,
	destination string,
	managedPath string,
	completedBytes int64,
	totalBytes int64,
	fallback bool,
) error {
	if err := downloadutils.ValidateDownloadURL(artifact.URL); err != nil {
		return fmt.Errorf("invalid artifact url for %s: %w", managedPath, err)
	}
	if err := downloadutils.ValidateChecksumFields("sha256", strings.ToLower(artifact.SHA256)); err != nil {
		return fmt.Errorf("invalid artifact checksum for %s: %w", managedPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if err := downloader.Download(s.ctx, downloadutils.TransferRequest{
		URL:             artifact.URL,
		DestinationPath: destination,
		ExpectedSize:    artifact.Size,
		ChecksumAlgo:    "sha256",
		Checksum:        strings.ToLower(artifact.SHA256),
		Progress: func(progress downloadutils.Progress) {
			downloaded := completedBytes + progress.Downloaded
			percent := 0
			if totalBytes > 0 {
				percent = int(downloaded * 100 / totalBytes)
				if percent > 100 {
					percent = 100
				}
			}
			s.emitUpdateProgress(UpdateProgress{
				Phase:      "downloading",
				File:       managedPath,
				Downloaded: downloaded,
				Total:      totalBytes,
				Percent:    percent,
				Fallback:   fallback,
			})
		},
	}); err != nil {
		return fmt.Errorf("download %s: %w", managedPath, err)
	}
	return nil
}

func (s *UpdateService) replacePatchesWithFullDownloads(
	downloader *downloadutils.Downloader,
	selected []selectedUpdateFile,
	task *updateutils.Task,
) error {
	var fallbackTotal int64
	for i := range task.Files {
		if task.Files[i].Kind == updateutils.TaskFileKindPatch {
			fallbackTotal += selected[i].release.Full.Size
		}
	}
	var completed int64
	for i := range task.Files {
		if task.Files[i].Kind != updateutils.TaskFileKindPatch {
			continue
		}
		full := selected[i].release.Full
		destination := filepath.Join(task.WorkDir, "artifacts", artifactFileName(task.Files[i].Path, updateutils.TaskFileKindFull, full.Compression))
		if err := s.downloadUpdateArtifact(downloader, full, destination, task.Files[i].Path, completed, fallbackTotal, true); err != nil {
			return err
		}
		completed += full.Size
		task.Files[i].Kind = updateutils.TaskFileKindFull
		task.Files[i].ArtifactPath = destination
		task.Files[i].ArtifactSize = full.Size
		task.Files[i].ArtifactSHA256 = full.SHA256
		task.Files[i].Compression = full.Compression
		task.Files[i].SourceSHA256 = ""
	}
	return nil
}

func (s *UpdateService) emitUpdateProgress(progress UpdateProgress) {
	if s.ctx != nil {
		wailsruntime.EventsEmit(s.ctx, "update:progress", progress)
	}
}

func runUpdaterPrepare(updaterPath string, taskPath string, workDir string) error {
	command := exec.Command(updaterPath, "prepare", "--task", taskPath)
	command.Dir = workDir
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return err
		}
		return fmt.Errorf("%s: %w", message, err)
	}
	return nil
}

func startUpdaterCommit(updaterPath string, taskPath string, workDir string, elevated bool) error {
	args := []string{"commit", "--task", taskPath}
	var (
		started *processutils.StartedProcess
		err     error
	)
	if elevated {
		started, err = processutils.StartProcessElevated(updaterPath, args, workDir)
	} else {
		started, err = processutils.StartProcess(updaterPath, args, workDir)
	}
	if err != nil {
		return fmt.Errorf("start updater commit: %w", err)
	}
	if started != nil && started.Handle != 0 {
		_ = processutils.CloseProcessHandle(started.Handle)
	}
	return nil
}

func selectedArtifactTotal(selected []selectedUpdateFile) int64 {
	var total int64
	for _, item := range selected {
		total += artifactForTask(item.release, item.task.Kind).Size
	}
	return total
}

func artifactForTask(file updateutils.ReleaseFile, kind string) updateutils.Artifact {
	if kind == updateutils.TaskFileKindPatch && file.Patch != nil {
		return file.Patch.Artifact
	}
	return file.Full
}

func taskUsesPatch(task *updateutils.Task) bool {
	for _, file := range task.Files {
		if file.Kind == updateutils.TaskFileKindPatch {
			return true
		}
	}
	return false
}

func versionsEqual(left string, right string) bool {
	return strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(left), "v"), strings.TrimPrefix(strings.TrimSpace(right), "v"))
}

func artifactFileName(managedPath string, kind string, compression string) string {
	name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(managedPath)
	if kind == updateutils.TaskFileKindPatch {
		return name + ".zsdiff"
	}
	if compression == updateutils.ArtifactCompressionZstd {
		return name + ".zst"
	}
	return name + ".full"
}

func safeUpdatePathPart(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, value)
	if value == "" {
		return "unknown"
	}
	return value
}
