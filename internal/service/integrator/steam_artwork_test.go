package integrator

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"lunabox/internal/models"
	"os"
	"path/filepath"
	"testing"
)

func TestImportSteamShortcutArtworkWritesCoverImages(t *testing.T) {
	coverPath := createSteamArtworkTestPNG(t)
	withSteamArtworkCoverFinder(t, func(gameID string) (string, string, error) {
		if gameID != "game-1" {
			t.Fatalf("unexpected game ID %q", gameID)
		}
		return coverPath, "/local/covers/game-1.png", nil
	})

	steamRoot := t.TempDir()
	if err := importSteamShortcutArtwork(steamRoot, "123456", 987654321, models.Game{ID: "game-1"}); err != nil {
		t.Fatalf("importSteamShortcutArtwork() returned error: %v", err)
	}

	gridDir := filepath.Join(steamRoot, "userdata", "123456", "config", "grid")
	for _, name := range []string{"987654321p.jpg", "987654321.jpg"} {
		path := filepath.Join(gridDir, name)
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("open Steam artwork %s: %v", name, err)
		}
		_, format, err := image.Decode(file)
		_ = file.Close()
		if err != nil {
			t.Fatalf("decode Steam artwork %s: %v", name, err)
		}
		if format != "jpeg" {
			t.Fatalf("Steam artwork %s format = %q, want jpeg", name, format)
		}
	}
}

func TestImportSteamShortcutArtworkSkipsMissingCover(t *testing.T) {
	withSteamArtworkCoverFinder(t, func(string) (string, string, error) {
		return "", "", nil
	})

	steamRoot := t.TempDir()
	if err := importSteamShortcutArtwork(steamRoot, "123456", 987654321, models.Game{ID: "game-1"}); err != nil {
		t.Fatalf("importSteamShortcutArtwork() returned error: %v", err)
	}
	gridDir := filepath.Join(steamRoot, "userdata", "123456", "config", "grid")
	if _, err := os.Stat(gridDir); !os.IsNotExist(err) {
		t.Fatalf("grid directory stat error = %v, want not exist", err)
	}
}

func TestImportSteamShortcutArtworkPreservesExistingGridImage(t *testing.T) {
	coverPath := createSteamArtworkTestPNG(t)
	withSteamArtworkCoverFinder(t, func(string) (string, string, error) {
		return coverPath, "/local/covers/game-1.png", nil
	})

	steamRoot := t.TempDir()
	gridDir := filepath.Join(steamRoot, "userdata", "123456", "config", "grid")
	if err := os.MkdirAll(gridDir, 0o755); err != nil {
		t.Fatalf("create grid directory: %v", err)
	}
	landscapePath := filepath.Join(gridDir, "987654321.jpg")
	existing := []byte("existing steam artwork")
	if err := os.WriteFile(landscapePath, existing, 0o644); err != nil {
		t.Fatalf("write existing grid artwork: %v", err)
	}

	if err := importSteamShortcutArtwork(steamRoot, "123456", 987654321, models.Game{ID: "game-1"}); err != nil {
		t.Fatalf("importSteamShortcutArtwork() returned error: %v", err)
	}

	got, err := os.ReadFile(landscapePath)
	if err != nil {
		t.Fatalf("read existing grid artwork: %v", err)
	}
	if !bytes.Equal(got, existing) {
		t.Fatalf("existing grid artwork was overwritten")
	}
	if _, err := os.Stat(filepath.Join(gridDir, "987654321p.jpg")); err != nil {
		t.Fatalf("portrait artwork was not written: %v", err)
	}
}

func withSteamArtworkCoverFinder(t *testing.T, fn func(string) (string, string, error)) {
	t.Helper()
	original := findManagedSteamArtworkCover
	findManagedSteamArtworkCover = fn
	t.Cleanup(func() {
		findManagedSteamArtworkCover = original
	})
}

func createSteamArtworkTestPNG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(40 * x), G: uint8(30 * y), B: 180, A: 255})
		}
	}

	path := filepath.Join(t.TempDir(), "cover.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create cover: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode cover: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close cover: %v", err)
	}
	return path
}

func TestEncodeSteamShortcutArtworkJPEGRejectsInvalidImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.txt")
	if err := os.WriteFile(path, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write invalid cover: %v", err)
	}
	if _, err := encodeSteamShortcutArtworkJPEG(path); err == nil {
		t.Fatalf("encodeSteamShortcutArtworkJPEG() returned nil error for invalid image")
	}
}

func TestEncodeSteamShortcutArtworkJPEGKeepsJPEGReadable(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})

	path := filepath.Join(t.TempDir(), "cover.jpg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg cover: %v", err)
	}
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		_ = file.Close()
		t.Fatalf("encode jpeg cover: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close jpeg cover: %v", err)
	}

	data, err := encodeSteamShortcutArtworkJPEG(path)
	if err != nil {
		t.Fatalf("encodeSteamShortcutArtworkJPEG() returned error: %v", err)
	}
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode encoded jpeg: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("encoded format = %q, want jpeg", format)
	}
}
