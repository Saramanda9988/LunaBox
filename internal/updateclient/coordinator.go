package updateclient

import (
	"context"
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

	"lunabox/internal/appconf"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/processutils"
	"lunabox/updater/updateutils"

	"github.com/google/uuid"
)

const (
	updateManifestMaxBytes = 4 * 1024 * 1024
	updaterExecutableName  = "LunaBoxUpdater.exe"
)

type Progress struct {
	Phase      string `json:"phase"`
	File       string `json:"file,omitempty"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Percent    int    `json:"percent"`
	Fallback   bool   `json:"fallback"`
}

type Result struct {
	Started      bool `json:"started"`
	FallbackUsed bool `json:"fallback_used"`
	FileCount    int  `json:"file_count"`
}

type selectedUpdateFile struct {
	release updateutils.ReleaseFile
	task    updateutils.TaskFile
}

type Options struct {
	ManifestURL     string
	CurrentVersion  string
	BuildMode       string
	UserAgent       string
	Config          *appconf.AppConfig
	CompareVersions func(currentVersion string, targetVersion string) (bool, error)
	Progress        func(Progress)
}

// Apply downloads verified update artifacts, prepares the transaction, and
// starts the standalone updater in commit mode.
func Apply(ctx context.Context, options Options) (*Result, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("in-app updates are currently supported on Windows only")
	}
	if ctx == nil || options.Config == nil || options.CompareVersions == nil {
		return nil, fmt.Errorf("update client is not initialized")
	}
	if err := downloadutils.ValidateDownloadURL(options.ManifestURL); err != nil {
		return nil, fmt.Errorf("invalid update manifest url: %w", err)
	}

	manifest, channel, err := fetchReleaseManifest(ctx, options.ManifestURL, options.BuildMode, options.Config, options.UserAgent)
	if err != nil {
		return nil, err
	}
	hasUpdate, err := options.CompareVersions(options.CurrentVersion, manifest.Version)
	if err != nil {
		return nil, fmt.Errorf("compare update manifest version: %w", err)
	}
	if !hasUpdate {
		return nil, fmt.Errorf("update manifest version %s is not newer than %s", manifest.Version, options.CurrentVersion)
	}
	channelName := fmt.Sprintf("windows-%s-%s", runtime.GOARCH, options.BuildMode)

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

	selected, err := selectUpdateFiles(channel, appDir, options.CurrentVersion, workDir)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return &Result{Started: false}, nil
	}
	_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL,
		newTelemetryEvent("update_available", transactionID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode))

	downloader, _, err := downloadutils.NewDownloader(downloadutils.TransferConfig{
		ProxyConfig: options.Config,
		UserAgent:   options.UserAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("create update downloader: %w", err)
	}

	task := &updateutils.Task{
		SchemaVersion: updateutils.TaskSchemaVersion,
		TransactionID: transactionID,
		TargetVersion: manifest.Version,
		BuildMode:     options.BuildMode,
		AppDir:        appDir,
		WorkDir:       workDir,
		WaitPID:       os.Getpid(),
		WaitTimeout:   600,
		RestartPath:   "LunaBox.exe",
		Files:         make([]updateutils.TaskFile, 0, len(selected)),
	}

	totalBytes := selectedArtifactTotal(selected)
	downloadStarted := newTelemetryEvent("download_started", transactionID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
	downloadStarted.TransferredBytes = totalBytes
	_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, downloadStarted)
	var completedBytes int64
	for i := range selected {
		item := &selected[i]
		artifact := artifactForTask(item.release, item.task.Kind)
		if err := downloadUpdateArtifact(ctx, options.Progress, downloader, artifact, item.task.ArtifactPath, item.task.Path, completedBytes, totalBytes, false); err != nil {
			return nil, err
		}
		completedBytes += artifact.Size
		task.Files = append(task.Files, item.task)
	}
	downloadVerified := newTelemetryEvent("download_verified", transactionID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
	downloadVerified.TransferredBytes = completedBytes
	_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, downloadVerified)

	taskPath := filepath.Join(workDir, "task.json")
	if err := updateutils.WriteTask(taskPath, task); err != nil {
		return nil, fmt.Errorf("write updater task: %w", err)
	}
	emitProgress(options.Progress, Progress{Phase: "preparing", Total: totalBytes, Downloaded: totalBytes, Percent: 100})
	prepareErr := runUpdaterPrepare(runnerPath, taskPath, workDir)
	fallbackUsed := false
	if prepareErr != nil && taskUsesPatch(task) {
		fallbackUsed = true
		emitProgress(options.Progress, Progress{Phase: "fallback", Fallback: true})
		if err := replacePatchesWithFullDownloads(ctx, options.Progress, downloader, selected, task); err != nil {
			return nil, fmt.Errorf("patch prepare failed (%v), and full fallback failed: %w", prepareErr, err)
		}
		if err := updateutils.WriteTask(taskPath, task); err != nil {
			return nil, fmt.Errorf("write full fallback task: %w", err)
		}
		prepareErr = runUpdaterPrepare(runnerPath, taskPath, workDir)
	}
	if prepareErr != nil {
		failedEvent := newTelemetryEvent("install_failed", transactionID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
		failedEvent.FailureCode = "prepare_failed"
		_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, failedEvent)
		return nil, fmt.Errorf("prepare update: %w", prepareErr)
	}

	emitProgress(options.Progress, Progress{Phase: "ready", Percent: 100, Fallback: fallbackUsed})
	pendingWritten := false
	if strings.TrimSpace(manifest.EventURL) != "" {
		if err := writePendingUpdate(pendingUpdate{
			EventID:        uuid.NewString(),
			EventURL:       manifest.EventURL,
			WorkDir:        workDir,
			TransactionID:  transactionID,
			CurrentVersion: options.CurrentVersion,
			TargetVersion:  manifest.Version,
			Channel:        channelName,
			BuildMode:      options.BuildMode,
		}); err == nil {
			pendingWritten = true
		}
	}
	if err := startUpdaterCommit(runnerPath, taskPath, workDir, options.BuildMode == "installer"); err != nil {
		if pendingWritten {
			removePendingUpdate()
		}
		failedEvent := newTelemetryEvent("install_failed", transactionID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
		failedEvent.FailureCode = "commit_start_failed"
		_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, failedEvent)
		return nil, err
	}

	result := &Result{
		Started:      true,
		FallbackUsed: fallbackUsed,
		FileCount:    len(task.Files),
	}
	return result, nil
}

func fetchReleaseManifest(
	ctx context.Context,
	manifestURL string,
	buildMode string,
	config *appconf.AppConfig,
	userAgent string,
) (*updateutils.ReleaseManifest, updateutils.ReleaseChannel, error) {
	client, _, err := downloadutils.NewSecureHTTPClientFromConfig(20*time.Second, config)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, fmt.Errorf("create update manifest client: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, updateutils.ReleaseChannel{}, err
	}
	request.Header.Set("User-Agent", userAgent)
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
	channelName := fmt.Sprintf("windows-%s-%s", runtime.GOARCH, buildMode)
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

func downloadUpdateArtifact(
	ctx context.Context,
	progressCallback func(Progress),
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
	if err := downloader.Download(ctx, downloadutils.TransferRequest{
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
			emitProgress(progressCallback, Progress{
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

func replacePatchesWithFullDownloads(
	ctx context.Context,
	progressCallback func(Progress),
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
		if err := downloadUpdateArtifact(ctx, progressCallback, downloader, full, destination, task.Files[i].Path, completed, fallbackTotal, true); err != nil {
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

func emitProgress(callback func(Progress), progress Progress) {
	if callback != nil {
		callback(progress)
	}
}

func runUpdaterPrepare(updaterPath string, taskPath string, workDir string) error {
	command := exec.Command(updaterPath, "prepare", "--task", taskPath)
	command.Dir = workDir
	configureUpdateHelperCommand(command)
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
		started, err = processutils.StartProcessElevatedHidden(updaterPath, args, workDir)
	} else {
		started, err = processutils.StartProcessHidden(updaterPath, args, workDir)
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
