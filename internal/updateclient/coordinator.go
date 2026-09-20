package updateclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	"lunabox/internal/utils/apputils"
	"lunabox/internal/utils/downloadutils"
	"lunabox/internal/utils/processutils"
	"lunabox/updater/updateutils"

	"github.com/google/uuid"
)

const (
	updateManifestMaxBytes = 4 * 1024 * 1024
	updaterExecutableName  = "LunaBoxUpdater.exe"
	patchSelectionRatio    = 0.80
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
	patches []selectedPatch
}

type selectedPatch struct {
	patch        updateutils.PatchArtifact
	artifactPath string
	targetSHA    string
	targetSize   int64
}

type releaseManifestResolver func(version string) (*updateutils.ReleaseManifest, updateutils.ReleaseChannel, error)

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
	installationID, installationIDErr := loadOrCreateInstallationID()
	if installationIDErr != nil {
		applog.LogWarningf(ctx, "create update telemetry installation id: %v", installationIDErr)
	}
	// reportFailure reports an install failure with its stage code and normalized
	// reason. The caller logs the raw error locally; nothing but the normalized
	// reason leaves the machine.
	reportFailure := func(stageCode string, reason string) {
		failedEvent := newTelemetryEvent("install_failed", transactionID, installationID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
		failedEvent.FailureCode = stageCode
		failedEvent.FailureReason = reason
		_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, failedEvent)
		applog.LogErrorf(ctx, "update install failed: %s/%s", stageCode, reason)
	}
	runnerPath := filepath.Join(workDir, "runner", updaterExecutableName)
	if err := apputils.CopyFile(installedUpdater, runnerPath); err != nil {
		return nil, fmt.Errorf("copy updater to transaction directory: %w", err)
	}

	manifestResolver := newReleaseManifestResolver(ctx, options.ManifestURL, options.BuildMode, options.Config, options.UserAgent)
	selected, err := selectUpdateFilesWithResolver(channel, appDir, options.CurrentVersion, workDir, manifestResolver)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return &Result{Started: false}, nil
	}
	_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL,
		newTelemetryEvent("update_available", transactionID, installationID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode))

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
	downloadStarted := newTelemetryEvent("download_started", transactionID, installationID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
	downloadStarted.TransferredBytes = totalBytes
	_ = reportEvent(ctx, options.Config, options.UserAgent, manifest.EventURL, downloadStarted)
	var completedBytes int64
	for i := range selected {
		item := &selected[i]
		if len(item.patches) == 0 {
			artifact := artifactForTask(item.release, item.task.Kind)
			if err := downloadUpdateArtifact(ctx, options.Progress, downloader, artifact, item.task.ArtifactPath, item.task.Path, completedBytes, totalBytes, false); err != nil {
				return nil, err
			}
			completedBytes += artifact.Size
		} else {
			for patchIndex := range item.patches {
				patch := &item.patches[patchIndex]
				if err := downloadUpdateArtifact(ctx, options.Progress, downloader, patch.patch.Artifact, patch.artifactPath, item.task.Path, completedBytes, totalBytes, false); err != nil {
					return nil, err
				}
				completedBytes += patch.patch.Artifact.Size
			}
		}
		task.Files = append(task.Files, item.task)
	}
	downloadVerified := newTelemetryEvent("download_verified", transactionID, installationID, options.CurrentVersion, manifest.Version, channelName, options.BuildMode)
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
			reportFailure("prepare_failed", failureReasonFallbackDownload)
			return nil, fmt.Errorf("patch prepare failed (%v), and full fallback failed: %w", prepareErr, err)
		}
		if err := updateutils.WriteTask(taskPath, task); err != nil {
			return nil, fmt.Errorf("write full fallback task: %w", err)
		}
		prepareErr = runUpdaterPrepare(runnerPath, taskPath, workDir)
	}
	if prepareErr != nil {
		reportFailure("prepare_failed", prepareFailureReason(workDir))
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
			InstallationID: installationID,
			CurrentVersion: options.CurrentVersion,
			TargetVersion:  manifest.Version,
			Channel:        channelName,
			BuildMode:      options.BuildMode,
		}); err == nil {
			pendingWritten = true
		}
	}
	if err := startUpdaterCommit(runnerPath, taskPath, workDir); err != nil {
		if pendingWritten {
			removePendingUpdate()
		}
		reportFailure("commit_start_failed", launchFailureReason(err))
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
	return selectUpdateFilesWithResolver(channel, appDir, currentVersion, workDir, nil)
}

func selectUpdateFilesWithResolver(channel updateutils.ReleaseChannel, appDir string, currentVersion string, workDir string, resolve releaseManifestResolver) ([]selectedUpdateFile, error) {
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
		var patches []selectedPatch
		if present {
			if chain, ok, err := resolvePatchChain(releaseFile, currentVersion, currentSHA, resolve); err != nil {
				return nil, fmt.Errorf("resolve patch chain for %s: %w", releaseFile.Path, err)
			} else if ok {
				kind = updateutils.TaskFileKindPatch
				first := chain[0]
				artifact = first.patch.Artifact
				sourceSHA = first.patch.SourceSHA256
				patches = make([]selectedPatch, len(chain))
				for index, patch := range chain {
					patches[index] = selectedPatch{
						patch:        patch.patch,
						artifactPath: filepath.Join(workDir, "artifacts", artifactFileName(releaseFile.Path, fmt.Sprintf("patch-%d", index), patch.patch.Compression)),
						targetSHA:    chain[index].targetSHA,
						targetSize:   chain[index].targetSize,
					}
				}
				if len(patches) > 1 {
					artifact = patches[0].patch.Artifact
				}
			}
		}
		artifactPath := filepath.Join(workDir, "artifacts", artifactFileName(releaseFile.Path, kind, artifact.Compression))
		if len(patches) > 0 {
			artifactPath = patches[0].artifactPath
		}
		selected = append(selected, selectedUpdateFile{
			release: releaseFile,
			patches: patches,
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
		if len(patches) > 1 {
			selected[len(selected)-1].task.PatchChain = make([]updateutils.TaskPatch, len(patches))
			for index := range patches {
				patch := patches[index]
				selected[len(selected)-1].task.PatchChain[index] = updateutils.TaskPatch{
					ArtifactPath:   patch.artifactPath,
					ArtifactSize:   patch.patch.Artifact.Size,
					ArtifactSHA256: patch.patch.Artifact.SHA256,
					Compression:    patch.patch.Artifact.Compression,
					SourceSHA256:   patch.patch.SourceSHA256,
					TargetSHA256:   patch.targetSHA,
					TargetSize:     patch.targetSize,
				}
			}
		}
	}
	return selected, nil
}

type patchStep struct {
	patch      updateutils.PatchArtifact
	targetSHA  string
	targetSize int64
}

func resolvePatchChain(file updateutils.ReleaseFile, currentVersion string, currentSHA string, resolve releaseManifestResolver) ([]patchStep, bool, error) {
	if file.Patch == nil {
		return nil, false, nil
	}
	if strings.EqualFold(currentSHA, file.Patch.SourceSHA256) && versionsEqual(currentVersion, file.Patch.SourceVersion) {
		chain := []patchStep{{patch: *file.Patch, targetSHA: file.TargetSHA256, targetSize: file.TargetSize}}
		return chain, patchChainIsSmaller(file, chain), nil
	}
	if resolve == nil {
		return nil, false, nil
	}

	reverse := []patchStep{{patch: *file.Patch, targetSHA: file.TargetSHA256, targetSize: file.TargetSize}}
	versionCursor := strings.TrimSpace(file.Patch.SourceVersion)
	seenVersions := map[string]struct{}{strings.ToLower(versionCursor): {}}
	for len(reverse) < 32 {
		manifest, channel, err := resolve(versionCursor)
		if err != nil || manifest == nil || !versionsEqual(manifest.Version, versionCursor) {
			return nil, false, nil
		}
		previousFile, ok := findReleaseFile(channel, file.Path)
		if !ok || previousFile.Patch == nil {
			return nil, false, nil
		}
		last := reverse[len(reverse)-1]
		if !strings.EqualFold(previousFile.TargetSHA256, last.patch.SourceSHA256) {
			return nil, false, nil
		}
		reverse = append(reverse, patchStep{
			patch:      *previousFile.Patch,
			targetSHA:  previousFile.TargetSHA256,
			targetSize: previousFile.TargetSize,
		})
		versionCursor = strings.TrimSpace(previousFile.Patch.SourceVersion)
		key := strings.ToLower(versionCursor)
		if _, seen := seenVersions[key]; seen {
			return nil, false, nil
		}
		seenVersions[key] = struct{}{}
		if versionsEqual(versionCursor, currentVersion) {
			if !strings.EqualFold(previousFile.Patch.SourceSHA256, currentSHA) {
				return nil, false, nil
			}
			for left, right := 0, len(reverse)-1; left < right; left, right = left+1, right-1 {
				reverse[left], reverse[right] = reverse[right], reverse[left]
			}
			return reverse, patchChainIsSmaller(file, reverse), nil
		}
	}
	return nil, false, nil
}

func patchChainIsSmaller(file updateutils.ReleaseFile, chain []patchStep) bool {
	if len(chain) == 0 || file.Full.Size <= 0 {
		return false
	}
	var patchBytes int64
	for _, step := range chain {
		patchBytes += step.patch.Artifact.Size
	}
	return float64(patchBytes) < float64(file.Full.Size)*patchSelectionRatio
}

func findReleaseFile(channel updateutils.ReleaseChannel, managedPath string) (updateutils.ReleaseFile, bool) {
	for _, file := range channel.Files {
		if strings.EqualFold(file.Path, managedPath) {
			return file, true
		}
	}
	return updateutils.ReleaseFile{}, false
}

func newReleaseManifestResolver(ctx context.Context, manifestURL string, buildMode string, config *appconf.AppConfig, userAgent string) releaseManifestResolver {
	return func(version string) (*updateutils.ReleaseManifest, updateutils.ReleaseChannel, error) {
		versionURL, err := manifestURLForVersion(manifestURL, version)
		if err != nil {
			return nil, updateutils.ReleaseChannel{}, err
		}
		return fetchReleaseManifest(ctx, versionURL, buildMode, config, userAgent)
	}
}

func manifestURLForVersion(manifestURL string, version string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(manifestURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("manifest url must use https")
	}
	const marker = "/v1/releases/"
	index := strings.LastIndex(parsed.Path, marker)
	if index < 0 {
		return "", fmt.Errorf("manifest url does not expose versioned release path")
	}
	remainder := parsed.Path[index+len(marker):]
	parts := strings.SplitN(remainder, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] != "manifest" {
		return "", fmt.Errorf("manifest url does not expose versioned release path")
	}
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" || strings.ContainsAny(version, `/\\`) {
		return "", fmt.Errorf("invalid release version")
	}
	parsed.Path = parsed.Path[:index+len(marker)] + version + "/manifest"
	parsed.RawPath = ""
	return parsed.String(), nil
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
		task.Files[i].PatchChain = nil
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

func startUpdaterCommit(updaterPath string, taskPath string, workDir string) error {
	args := []string{"commit", "--task", taskPath}
	started, err := processutils.StartProcessHidden(updaterPath, args, workDir)
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
		if len(item.patches) > 0 {
			for _, patch := range item.patches {
				total += patch.patch.Artifact.Size
			}
			continue
		}
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
	if strings.HasPrefix(kind, updateutils.TaskFileKindPatch+"-") {
		return name + "." + strings.TrimPrefix(kind, updateutils.TaskFileKindPatch+"-") + ".zsdiff"
	}
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
