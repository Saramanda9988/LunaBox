package updateutils

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

const (
	ManifestSchemaVersion = 1
	TaskSchemaVersion     = 1

	InstallPolicyAlways    = "always"
	InstallPolicyIfPresent = "if-present"

	ArtifactCompressionNone = "none"
	ArtifactCompressionZstd = "zstd"

	TaskFileKindPatch = "patch"
	TaskFileKindFull  = "full"
)

var managedPaths = map[string]struct{}{
	"lunabox.exe":        {},
	"lunaboxupdater.exe": {},
	"lunacli.exe":        {},
	"duckdb.dll":         {},
	"7z/7z.exe":          {},
	"7z/7z.dll":          {},
}

// ReleaseManifest describes the platform-specific update assets published with
// a LunaBox release. Every channel contains complete-file fallbacks; patches are
// optional accelerators for one exact source binary.
type ReleaseManifest struct {
	SchemaVersion int                       `json:"schema_version"`
	Version       string                    `json:"version"`
	Channels      map[string]ReleaseChannel `json:"channels"`
}

type ReleaseChannel struct {
	Files []ReleaseFile `json:"files"`
}

type ReleaseFile struct {
	Path          string         `json:"path"`
	InstallPolicy string         `json:"install_policy,omitempty"`
	TargetSHA256  string         `json:"target_sha256"`
	TargetSize    int64          `json:"target_size"`
	Full          Artifact       `json:"full"`
	Patch         *PatchArtifact `json:"patch,omitempty"`
}

type Artifact struct {
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Compression string `json:"compression,omitempty"`
}

type PatchArtifact struct {
	Artifact
	SourceVersion string `json:"source_version"`
	SourceSHA256  string `json:"source_sha256"`
}

// Task is the local handoff contract between LunaBox and LunaBoxUpdater.
// Network URLs are intentionally absent: LunaBox downloads and verifies every
// artifact before invoking the updater.
type Task struct {
	SchemaVersion int        `json:"schema_version"`
	TransactionID string     `json:"transaction_id"`
	TargetVersion string     `json:"target_version"`
	BuildMode     string     `json:"build_mode"`
	AppDir        string     `json:"app_dir"`
	WorkDir       string     `json:"work_dir"`
	WaitPID       int        `json:"wait_pid"`
	WaitTimeout   int        `json:"wait_timeout_seconds,omitempty"`
	RestartPath   string     `json:"restart_path"`
	RestartArgs   []string   `json:"restart_args,omitempty"`
	Files         []TaskFile `json:"files"`
}

type TaskFile struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	ArtifactPath   string `json:"artifact_path"`
	ArtifactSize   int64  `json:"artifact_size"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	Compression    string `json:"compression,omitempty"`
	SourceSHA256   string `json:"source_sha256,omitempty"`
	TargetSHA256   string `json:"target_sha256"`
	TargetSize     int64  `json:"target_size"`
}

func (m ReleaseManifest) Validate(channelName string) (ReleaseChannel, error) {
	if m.SchemaVersion != ManifestSchemaVersion {
		return ReleaseChannel{}, fmt.Errorf("unsupported update manifest schema: %d", m.SchemaVersion)
	}
	if strings.TrimSpace(m.Version) == "" {
		return ReleaseChannel{}, fmt.Errorf("update manifest version is required")
	}
	channel, ok := m.Channels[channelName]
	if !ok {
		return ReleaseChannel{}, fmt.Errorf("update channel %q is not available", channelName)
	}
	if len(channel.Files) == 0 {
		return ReleaseChannel{}, fmt.Errorf("update channel %q has no files", channelName)
	}

	seen := make(map[string]struct{}, len(channel.Files))
	for i := range channel.Files {
		file := &channel.Files[i]
		normalized, err := NormalizeManagedPath(file.Path)
		if err != nil {
			return ReleaseChannel{}, fmt.Errorf("file %d: %w", i, err)
		}
		file.Path = normalized
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			return ReleaseChannel{}, fmt.Errorf("duplicate update file path: %s", normalized)
		}
		seen[key] = struct{}{}

		if file.InstallPolicy == "" {
			file.InstallPolicy = InstallPolicyAlways
		}
		if file.InstallPolicy != InstallPolicyAlways && file.InstallPolicy != InstallPolicyIfPresent {
			return ReleaseChannel{}, fmt.Errorf("file %s has invalid install policy %q", normalized, file.InstallPolicy)
		}
		if err := validateSHA256(file.TargetSHA256, "target_sha256"); err != nil {
			return ReleaseChannel{}, fmt.Errorf("file %s: %w", normalized, err)
		}
		if file.TargetSize <= 0 {
			return ReleaseChannel{}, fmt.Errorf("file %s target_size must be positive", normalized)
		}
		if err := file.Full.Validate(); err != nil {
			return ReleaseChannel{}, fmt.Errorf("file %s full artifact: %w", normalized, err)
		}
		if file.Patch != nil {
			if err := file.Patch.Artifact.Validate(); err != nil {
				return ReleaseChannel{}, fmt.Errorf("file %s patch artifact: %w", normalized, err)
			}
			if file.Patch.Compression != ArtifactCompressionZstd {
				return ReleaseChannel{}, fmt.Errorf("file %s patch must use zstd compression", normalized)
			}
			if strings.TrimSpace(file.Patch.SourceVersion) == "" {
				return ReleaseChannel{}, fmt.Errorf("file %s patch source_version is required", normalized)
			}
			if err := validateSHA256(file.Patch.SourceSHA256, "source_sha256"); err != nil {
				return ReleaseChannel{}, fmt.Errorf("file %s patch: %w", normalized, err)
			}
		}
	}

	return channel, nil
}

func (a *Artifact) Validate() error {
	if a == nil {
		return fmt.Errorf("artifact is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(a.URL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("artifact url must use https")
	}
	if a.Size <= 0 {
		return fmt.Errorf("artifact size must be positive")
	}
	if err := validateSHA256(a.SHA256, "sha256"); err != nil {
		return err
	}
	if a.Compression == "" {
		a.Compression = ArtifactCompressionNone
	}
	if a.Compression != ArtifactCompressionNone && a.Compression != ArtifactCompressionZstd {
		return fmt.Errorf("unsupported artifact compression: %s", a.Compression)
	}
	return nil
}

func (t *Task) Validate() error {
	if t.SchemaVersion != TaskSchemaVersion {
		return fmt.Errorf("unsupported update task schema: %d", t.SchemaVersion)
	}
	if !isSafeIdentifier(t.TransactionID) {
		return fmt.Errorf("invalid transaction id")
	}
	if strings.TrimSpace(t.TargetVersion) == "" {
		return fmt.Errorf("target version is required")
	}
	if t.BuildMode != "portable" && t.BuildMode != "installer" {
		return fmt.Errorf("invalid build mode: %s", t.BuildMode)
	}
	if !filepath.IsAbs(t.AppDir) || !filepath.IsAbs(t.WorkDir) {
		return fmt.Errorf("app_dir and work_dir must be absolute")
	}
	if t.WaitPID <= 0 {
		return fmt.Errorf("wait_pid must be positive")
	}
	restartPath, err := NormalizeManagedPath(t.RestartPath)
	if err != nil {
		return fmt.Errorf("restart path: %w", err)
	}
	if !strings.EqualFold(restartPath, "LunaBox.exe") {
		return fmt.Errorf("restart path must be LunaBox.exe")
	}
	t.RestartPath = restartPath
	if len(t.Files) == 0 {
		return fmt.Errorf("update task has no files")
	}

	seen := make(map[string]struct{}, len(t.Files))
	for i := range t.Files {
		file := &t.Files[i]
		normalized, err := NormalizeManagedPath(file.Path)
		if err != nil {
			return fmt.Errorf("task file %d: %w", i, err)
		}
		file.Path = normalized
		key := strings.ToLower(normalized)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate task file path: %s", normalized)
		}
		seen[key] = struct{}{}

		if file.Kind != TaskFileKindPatch && file.Kind != TaskFileKindFull {
			return fmt.Errorf("file %s has invalid kind %q", normalized, file.Kind)
		}
		if !filepath.IsAbs(file.ArtifactPath) || !pathWithin(t.WorkDir, file.ArtifactPath) {
			return fmt.Errorf("file %s artifact must be inside work_dir", normalized)
		}
		if file.ArtifactSize <= 0 {
			return fmt.Errorf("file %s artifact_size must be positive", normalized)
		}
		if err := validateSHA256(file.ArtifactSHA256, "artifact_sha256"); err != nil {
			return fmt.Errorf("file %s: %w", normalized, err)
		}
		if err := validateSHA256(file.TargetSHA256, "target_sha256"); err != nil {
			return fmt.Errorf("file %s: %w", normalized, err)
		}
		if file.TargetSize <= 0 {
			return fmt.Errorf("file %s target_size must be positive", normalized)
		}
		if file.Compression == "" {
			file.Compression = ArtifactCompressionNone
		}
		if file.Compression != ArtifactCompressionNone && file.Compression != ArtifactCompressionZstd {
			return fmt.Errorf("file %s has invalid compression %q", normalized, file.Compression)
		}
		if file.Kind == TaskFileKindPatch {
			if file.Compression != ArtifactCompressionZstd {
				return fmt.Errorf("file %s patch must use zstd compression", normalized)
			}
			if err := validateSHA256(file.SourceSHA256, "source_sha256"); err != nil {
				return fmt.Errorf("file %s: %w", normalized, err)
			}
		}
	}
	return nil
}

func NormalizeManagedPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || strings.HasPrefix(value, "/") || filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", fmt.Errorf("invalid managed path: %q", value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, ":") {
		return "", fmt.Errorf("invalid managed path: %q", value)
	}
	if _, ok := managedPaths[strings.ToLower(cleaned)]; !ok {
		return "", fmt.Errorf("unmanaged update path: %s", cleaned)
	}
	return cleaned, nil
}

func validateSHA256(value string, field string) error {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return fmt.Errorf("%s must contain 64 hexadecimal characters", field)
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return fmt.Errorf("%s must be hexadecimal", field)
		}
	}
	return nil
}

func isSafeIdentifier(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func pathWithin(root string, candidate string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func localPath(root string, managedPath string) string {
	return filepath.Join(root, filepath.FromSlash(managedPath))
}

func stagingDir(task *Task) string {
	return filepath.Join(task.WorkDir, "staging")
}
