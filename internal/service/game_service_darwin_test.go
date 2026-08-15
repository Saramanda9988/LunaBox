//go:build darwin

package service

import (
	"os"
	"path/filepath"
	"testing"

	"lunabox/internal/service/gamehelper"
)

func TestExecutableDialogDirectoryMacAppBundle(t *testing.T) {
	parent := t.TempDir()
	appPath := filepath.Join(parent, "Sample Game.app")
	if err := mkdirAppBundle(appPath); err != nil {
		t.Fatal(err)
	}

	defaultDir := gamehelper.ExecutableDialogDirectory(appPath)
	if defaultDir != parent {
		t.Fatalf("expected default dir %q, got %q", parent, defaultDir)
	}
}

func TestResolveExecutablePathForImportAcceptsMacAppBundle(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "Sample Game.app")
	if err := mkdirAppBundle(appPath); err != nil {
		t.Fatal(err)
	}

	service := &GameService{}
	resolved, err := service.ResolveExecutablePathForImport(appPath)
	if err != nil {
		t.Fatalf("resolve import path: %v", err)
	}
	if resolved != appPath {
		t.Fatalf("expected app bundle path %q, got %q", appPath, resolved)
	}
}

func TestResolveExecutablePathAcceptsMacAppBundle(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "Sample Game.app")
	if err := mkdirAppBundle(appPath); err != nil {
		t.Fatal(err)
	}

	service := &StartService{}
	resolved, processName, cancelled, err := service.resolveExecutablePath("game-id", appPath, "Existing")
	if err != nil {
		t.Fatalf("resolve executable path: %v", err)
	}
	if cancelled {
		t.Fatal("expected app bundle path to resolve without prompting")
	}
	if resolved != appPath {
		t.Fatalf("expected app bundle path %q, got %q", appPath, resolved)
	}
	if processName != "Existing" {
		t.Fatalf("expected existing process name, got %q", processName)
	}
}

func TestExecutableOpenDialogOptionsDarwinAllowsAppPackages(t *testing.T) {
	options := gamehelper.ExecutableOpenDialogOptions("Select", "/Applications")
	if len(options.Filters) != 0 {
		t.Fatalf("expected no filters on darwin, got %#v", options.Filters)
	}
	if options.TreatsFilePackagesAsDirectories {
		t.Fatal("expected app packages to remain selectable as package files")
	}
	if !options.ResolvesAliases {
		t.Fatal("expected aliases to resolve")
	}
}

func TestWineRunnerOpenDialogOptionsDarwinCanBrowseAppPackages(t *testing.T) {
	options := gamehelper.WineRunnerOpenDialogOptions("Select", "/Applications")
	if !options.TreatsFilePackagesAsDirectories {
		t.Fatal("expected wine runner selector to browse inside app packages")
	}
	if !options.ResolvesAliases {
		t.Fatal("expected aliases to resolve")
	}
}

func mkdirAppBundle(path string) error {
	return os.MkdirAll(filepath.Join(path, "Contents", "MacOS"), 0755)
}
