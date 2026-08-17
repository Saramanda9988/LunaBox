package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"lunabox/updater/updateutils"
)

func TestAssetURLUsesConfiguredBaseURL(t *testing.T) {
	t.Parallel()

	got := assetURL(options{
		assetBaseURL: "https://updates.example.com/releases/2.0.0/",
	}, "LunaBox.exe.zst")
	if got != "https://updates.example.com/releases/2.0.0/LunaBox.exe.zst" {
		t.Fatalf("unexpected asset URL: %s", got)
	}
}

func TestRunBuildsValidatedManifestAndFullFallbacks(t *testing.T) {
	t.Parallel()

	inputRoot := t.TempDir()
	outputDir := t.TempDir()
	version := "2.0.0"
	for _, arch := range []string{"amd64", "arm64"} {
		for _, mode := range []string{"portable", "installer"} {
			channel := fmt.Sprintf("windows-%s-%s", arch, mode)
			dir := filepath.Join(inputRoot, fmt.Sprintf("update-runtime-%s-%s", version, channel))
			writeRuntimeFixture(t, dir)
		}
	}

	if err := run(options{
		inputRoot:  inputRoot,
		outputDir:  outputDir,
		version:    version,
		repository: "example/LunaBox",
	}); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(outputDir, "LunaBox-2.0.0-update-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest updateutils.ReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Channels) != 4 {
		t.Fatalf("got %d channels, want 4", len(manifest.Channels))
	}
	for name := range manifest.Channels {
		channel, err := manifest.Validate(name)
		if err != nil {
			t.Fatalf("validate %s: %v", name, err)
		}
		if len(channel.Files) != 5 {
			t.Fatalf("channel %s has %d files, want 5", name, len(channel.Files))
		}
		for _, file := range channel.Files {
			assetName := filepath.Base(file.Full.URL)
			if _, err := os.Stat(filepath.Join(outputDir, assetName)); err != nil {
				t.Fatalf("full asset %s is missing: %v", assetName, err)
			}
		}
	}
}

func TestRunBuildsOnlySelectedArchitectures(t *testing.T) {
	t.Parallel()

	inputRoot := t.TempDir()
	outputDir := t.TempDir()
	version := "2.0.0-test"
	for _, mode := range []string{"portable", "installer"} {
		channel := fmt.Sprintf("windows-amd64-%s", mode)
		dir := filepath.Join(inputRoot, fmt.Sprintf("update-runtime-%s-%s", version, channel))
		writeRuntimeFixture(t, dir)
	}

	if err := run(options{
		inputRoot:     inputRoot,
		outputDir:     outputDir,
		version:       version,
		repository:    "example/LunaBox",
		architectures: "amd64",
	}); err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(outputDir, "LunaBox-2.0.0-test-update-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest updateutils.ReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Channels) != 2 {
		t.Fatalf("got %d channels, want 2", len(manifest.Channels))
	}
	for _, mode := range []string{"portable", "installer"} {
		name := fmt.Sprintf("windows-amd64-%s", mode)
		if _, ok := manifest.Channels[name]; !ok {
			t.Fatalf("channel %s is missing", name)
		}
	}
}

func TestBuildPatchUsesUpdaterCompatibleZstdFormat(t *testing.T) {
	if _, err := exec.LookPath("zstd"); err != nil {
		t.Skip("zstd CLI is not installed")
	}

	root := t.TempDir()
	previousRoot := filepath.Join(root, "previous")
	outputDir := filepath.Join(root, "output")
	if err := os.MkdirAll(previousRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldBytes := make([]byte, 2*1024*1024)
	for i := range oldBytes {
		oldBytes[i] = byte((i*31 + i/97) % 251)
	}
	newBytes := bytes.Clone(oldBytes)
	copy(newBytes[512*1024:512*1024+4096], bytes.Repeat([]byte("updated-block"), 342)[:4096])

	oldExe := filepath.Join(root, "old-LunaBox.exe")
	newExe := filepath.Join(root, "new-LunaBox.exe")
	if err := os.WriteFile(oldExe, oldBytes, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newExe, newBytes, 0755); err != nil {
		t.Fatal(err)
	}
	channelName := "windows-amd64-portable"
	previousAsset := filepath.Join(previousRoot, "LunaBox-1.0.0-"+channelName+"-LunaBox.exe.zst")
	if err := compressFull(oldExe, previousAsset); err != nil {
		t.Fatal(err)
	}
	fullAsset := filepath.Join(root, "new-full.zst")
	if err := compressFull(newExe, fullAsset); err != nil {
		t.Fatal(err)
	}
	_, fullSize, err := updateutils.FileSHA256(fullAsset)
	if err != nil {
		t.Fatal(err)
	}

	patch, err := buildPatch(options{
		previousRoot:    previousRoot,
		outputDir:       outputDir,
		version:         "1.1.0",
		previousVersion: "1.0.0",
		repository:      "example/LunaBox",
	}, channelName, newExe, fullSize)
	if err != nil {
		t.Fatal(err)
	}
	if patch == nil || patch.SourceSHA256 == "" {
		t.Fatal("expected a verified patch artifact")
	}
}

func writeRuntimeFixture(t *testing.T, root string) {
	t.Helper()
	for _, path := range []string{"LunaBox.exe", "LunaBoxUpdater.exe", "lunacli.exe", "7z/7z.exe", "7z/7z.dll"} {
		filePath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filePath, []byte("fixture-"+path), 0755); err != nil {
			t.Fatal(err)
		}
	}
}
