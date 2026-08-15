package integrator

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"lunabox/internal/models"
	"lunabox/internal/utils/imageutils"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

var findManagedSteamArtworkCover = imageutils.FindManagedCoverFile

func importSteamShortcutArtwork(steamRoot, userID string, appID uint32, game models.Game) error {
	steamRoot = strings.TrimSpace(steamRoot)
	userID = strings.TrimSpace(userID)
	if steamRoot == "" || userID == "" || appID == 0 || strings.TrimSpace(game.ID) == "" {
		return nil
	}

	coverPath, _, err := findManagedSteamArtworkCover(game.ID)
	if err != nil {
		return fmt.Errorf("find managed cover: %w", err)
	}
	if coverPath == "" {
		return nil
	}

	artwork, err := encodeSteamShortcutArtworkJPEG(coverPath)
	if err != nil {
		return fmt.Errorf("prepare Steam artwork: %w", err)
	}

	gridDir := filepath.Join(steamRoot, "userdata", userID, "config", "grid")
	if err := os.MkdirAll(gridDir, 0o755); err != nil {
		return fmt.Errorf("create Steam grid directory: %w", err)
	}

	baseName := strconv.FormatUint(uint64(appID), 10)
	if err := writeSteamShortcutArtworkFile(filepath.Join(gridDir, baseName+"p.jpg"), artwork); err != nil {
		return fmt.Errorf("write Steam portrait artwork: %w", err)
	}

	landscapePath := filepath.Join(gridDir, baseName+".jpg")
	if _, err := os.Stat(landscapePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat Steam grid artwork: %w", err)
	}
	if err := writeSteamShortcutArtworkFile(landscapePath, artwork); err != nil {
		return fmt.Errorf("write Steam grid artwork: %w", err)
	}
	return nil
}

func encodeSteamShortcutArtworkJPEG(path string) ([]byte, error) {
	src, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source image: %w", err)
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("decode source image: %w", err)
	}

	var output bytes.Buffer
	if err := jpeg.Encode(&output, img, &jpeg.Options{Quality: 92}); err != nil {
		return nil, fmt.Errorf("encode jpeg: %w", err)
	}
	return output.Bytes(), nil
}

func writeSteamShortcutArtworkFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return nil
}
