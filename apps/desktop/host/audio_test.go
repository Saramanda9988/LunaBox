package host

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/audio"
)

func TestMygoAudioDecodesWhaleMP3(t *testing.T) {
	assetsDir := filepath.Join("..", "..", "..", "packages", "plugin-sdk", "examples", "whale-pet", "assets")
	for _, name := range []string{"press.mp3", "release.mp3", "press2.mp3", "release2.mp3"} {
		p := filepath.Join(assetsDir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		snd, err := audio.DecodeSound(data)
		if err != nil {
			t.Fatalf("audio.DecodeSound(%s): %v", name, err)
		}
		if snd.Duration() <= 0 {
			t.Fatalf("expected positive duration for %s, got %v", name, snd.Duration())
		}
	}
}
