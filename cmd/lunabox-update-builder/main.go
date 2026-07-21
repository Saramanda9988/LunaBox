package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"lunabox/internal/utils/updateutils"

	"github.com/klauspost/compress/zstd"
)

const patchPublishRatio = 0.80

type options struct {
	inputRoot       string
	previousRoot    string
	outputDir       string
	version         string
	previousVersion string
	repository      string
}

type managedFileSpec struct {
	Path            string
	InstallerPolicy string
	PortablePolicy  string
}

var managedFileSpecs = []managedFileSpec{
	{Path: "LunaBox.exe", InstallerPolicy: updateutils.InstallPolicyAlways, PortablePolicy: updateutils.InstallPolicyAlways},
	{Path: "LunaBoxUpdater.exe", InstallerPolicy: updateutils.InstallPolicyAlways, PortablePolicy: updateutils.InstallPolicyAlways},
	{Path: "lunacli.exe", InstallerPolicy: updateutils.InstallPolicyIfPresent, PortablePolicy: updateutils.InstallPolicyAlways},
	{Path: "duckdb.dll", InstallerPolicy: updateutils.InstallPolicyAlways, PortablePolicy: updateutils.InstallPolicyAlways},
	{Path: "7z/7z.exe", InstallerPolicy: updateutils.InstallPolicyAlways, PortablePolicy: updateutils.InstallPolicyAlways},
	{Path: "7z/7z.dll", InstallerPolicy: updateutils.InstallPolicyAlways, PortablePolicy: updateutils.InstallPolicyAlways},
}

func main() {
	var opts options
	flag.StringVar(&opts.inputRoot, "input-root", "", "directory containing update-runtime artifacts")
	flag.StringVar(&opts.previousRoot, "previous-root", "", "directory containing previous full update assets")
	flag.StringVar(&opts.outputDir, "output", "", "release asset output directory")
	flag.StringVar(&opts.version, "version", "", "target version")
	flag.StringVar(&opts.previousVersion, "previous-version", "", "previous stable version")
	flag.StringVar(&opts.repository, "repository", "", "GitHub owner/repository")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "update asset builder:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.inputRoot == "" || opts.outputDir == "" || opts.version == "" || opts.repository == "" {
		return fmt.Errorf("--input-root, --output, --version, and --repository are required")
	}
	if err := os.MkdirAll(opts.outputDir, 0755); err != nil {
		return err
	}

	manifest := updateutils.ReleaseManifest{
		SchemaVersion: updateutils.ManifestSchemaVersion,
		Version:       opts.version,
		Channels:      make(map[string]updateutils.ReleaseChannel),
	}
	for _, arch := range []string{"amd64", "arm64"} {
		for _, mode := range []string{"portable", "installer"} {
			channelName := fmt.Sprintf("windows-%s-%s", arch, mode)
			inputDir := filepath.Join(opts.inputRoot, fmt.Sprintf("update-runtime-%s-%s", opts.version, channelName))
			channel, err := buildChannel(opts, channelName, mode, inputDir)
			if err != nil {
				return fmt.Errorf("build channel %s: %w", channelName, err)
			}
			manifest.Channels[channelName] = channel
		}
	}

	channelNames := make([]string, 0, len(manifest.Channels))
	for name := range manifest.Channels {
		channelNames = append(channelNames, name)
	}
	sort.Strings(channelNames)
	for _, name := range channelNames {
		if _, err := manifest.Validate(name); err != nil {
			return fmt.Errorf("validate generated channel %s: %w", name, err)
		}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(opts.outputDir, fmt.Sprintf("LunaBox-%s-update-manifest.json", opts.version))
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("generated %s with %d channels\n", manifestPath, len(manifest.Channels))
	return nil
}

func buildChannel(opts options, channelName string, mode string, inputDir string) (updateutils.ReleaseChannel, error) {
	if info, err := os.Stat(inputDir); err != nil || !info.IsDir() {
		return updateutils.ReleaseChannel{}, fmt.Errorf("runtime input directory not found: %s", inputDir)
	}

	channel := updateutils.ReleaseChannel{}
	for _, spec := range managedFileSpecs {
		sourcePath := filepath.Join(inputDir, filepath.FromSlash(spec.Path))
		info, err := os.Stat(sourcePath)
		if os.IsNotExist(err) {
			if spec.Path == "duckdb.dll" {
				continue
			}
			return updateutils.ReleaseChannel{}, fmt.Errorf("managed file is missing: %s", spec.Path)
		}
		if err != nil {
			return updateutils.ReleaseChannel{}, err
		}
		if info.IsDir() {
			return updateutils.ReleaseChannel{}, fmt.Errorf("managed path is a directory: %s", spec.Path)
		}

		targetSHA, targetSize, err := updateutils.FileSHA256(sourcePath)
		if err != nil {
			return updateutils.ReleaseChannel{}, err
		}
		assetBase := fmt.Sprintf("LunaBox-%s-%s-%s", opts.version, channelName, assetPathName(spec.Path))
		fullName := assetBase + ".zst"
		fullPath := filepath.Join(opts.outputDir, fullName)
		if err := compressFull(sourcePath, fullPath); err != nil {
			return updateutils.ReleaseChannel{}, fmt.Errorf("compress %s: %w", spec.Path, err)
		}
		fullSHA, fullSize, err := updateutils.FileSHA256(fullPath)
		if err != nil {
			return updateutils.ReleaseChannel{}, err
		}

		policy := spec.PortablePolicy
		if mode == "installer" {
			policy = spec.InstallerPolicy
		}
		releaseFile := updateutils.ReleaseFile{
			Path:          spec.Path,
			InstallPolicy: policy,
			TargetSHA256:  targetSHA,
			TargetSize:    targetSize,
			Full: updateutils.Artifact{
				URL:         releaseURL(opts.repository, opts.version, fullName),
				Size:        fullSize,
				SHA256:      fullSHA,
				Compression: updateutils.ArtifactCompressionZstd,
			},
		}

		if spec.Path == "LunaBox.exe" && opts.previousVersion != "" && opts.previousRoot != "" {
			patch, patchErr := buildPatch(opts, channelName, sourcePath, fullSize)
			if patchErr != nil {
				fmt.Fprintf(os.Stderr, "skipping patch for %s: %v\n", channelName, patchErr)
			} else {
				releaseFile.Patch = patch
			}
		}
		channel.Files = append(channel.Files, releaseFile)
	}
	return channel, nil
}

func buildPatch(opts options, channelName string, targetPath string, fullSize int64) (*updateutils.PatchArtifact, error) {
	previousFullName := fmt.Sprintf(
		"LunaBox-%s-%s-LunaBox.exe.zst",
		opts.previousVersion,
		channelName,
	)
	previousFullPath, err := findFileRecursively(opts.previousRoot, previousFullName)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "lunabox-update-base-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	previousExe := filepath.Join(tempDir, "LunaBox.exe")
	if err := decompressFull(previousFullPath, previousExe); err != nil {
		return nil, fmt.Errorf("decompress previous executable: %w", err)
	}
	sourceSHA, sourceSize, err := updateutils.FileSHA256(previousExe)
	if err != nil {
		return nil, err
	}

	patchName := fmt.Sprintf(
		"LunaBox-%s-%s-LunaBox.exe-from-%s.zsdiff",
		opts.version,
		channelName,
		opts.previousVersion,
	)
	patchPath := filepath.Join(opts.outputDir, patchName)
	args := []string{"--patch-from", previousExe, "--single-thread", "-19", "--force", "-o", patchPath}
	windowLog := highBit(uint64(sourceSize)) + 1
	if windowLog >= 27 {
		if windowLog > 30 {
			return nil, fmt.Errorf("source executable is too large for zstd patching")
		}
		args = append(args, fmt.Sprintf("--long=%d", windowLog))
	}
	args = append(args, targetPath)
	command := exec.Command("zstd", args...)
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("zstd --patch-from failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	verifiedTarget := filepath.Join(tempDir, "verified-LunaBox.exe")
	if err := updateutils.ReconstructZstdPatch(previousExe, patchPath, verifiedTarget); err != nil {
		_ = os.Remove(patchPath)
		return nil, fmt.Errorf("verify generated patch reconstruction: %w", err)
	}
	verifiedSHA, verifiedSize, err := updateutils.FileSHA256(verifiedTarget)
	if err != nil {
		_ = os.Remove(patchPath)
		return nil, err
	}
	targetSHA, targetSize, err := updateutils.FileSHA256(targetPath)
	if err != nil {
		_ = os.Remove(patchPath)
		return nil, err
	}
	if verifiedSize != targetSize || !strings.EqualFold(verifiedSHA, targetSHA) {
		_ = os.Remove(patchPath)
		return nil, fmt.Errorf("generated patch does not reconstruct the target executable")
	}
	patchSHA, patchSize, err := updateutils.FileSHA256(patchPath)
	if err != nil {
		return nil, err
	}
	if float64(patchSize) >= float64(fullSize)*patchPublishRatio {
		_ = os.Remove(patchPath)
		return nil, fmt.Errorf("patch is not beneficial (%d bytes vs %d-byte full asset)", patchSize, fullSize)
	}
	return &updateutils.PatchArtifact{
		Artifact: updateutils.Artifact{
			URL:         releaseURL(opts.repository, opts.version, patchName),
			Size:        patchSize,
			SHA256:      patchSHA,
			Compression: updateutils.ArtifactCompressionZstd,
		},
		SourceVersion: opts.previousVersion,
		SourceSHA256:  sourceSHA,
	}, nil
}

func compressFull(sourcePath string, destinationPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	encoder, err := zstd.NewWriter(output, zstd.WithEncoderLevel(zstd.SpeedBetterCompression), zstd.WithEncoderCRC(true))
	if err != nil {
		_ = output.Close()
		return err
	}
	_, copyErr := io.Copy(encoder, input)
	closeEncoderErr := encoder.Close()
	closeOutputErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeEncoderErr != nil {
		return closeEncoderErr
	}
	return closeOutputErr
}

func decompressFull(sourcePath string, destinationPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	decoder, err := zstd.NewReader(input, zstd.WithDecoderLowmem(true))
	if err != nil {
		return err
	}
	defer decoder.Close()
	output, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, decoder)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func findFileRecursively(root string, name string) (string, error) {
	var match string
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == name {
			match = current
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if match == "" {
		return "", fmt.Errorf("previous full asset not found: %s", name)
	}
	return match, nil
}

func releaseURL(repository string, version string, fileName string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", strings.Trim(repository, "/"), strings.TrimPrefix(version, "v"), fileName)
}

func assetPathName(managedPath string) string {
	return strings.NewReplacer("/", "_", "\\", "_").Replace(managedPath)
}

func highBit(value uint64) int {
	count := 0
	value >>= 1
	for value > 0 {
		value >>= 1
		count++
	}
	return count
}
