//go:build linux

package umbra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/utils/apputils"
)

func newCredentialStores(cfg Config) (umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	dir, err := credentialDir(cfg)
	if err != nil {
		return nil, nil, err
	}
	return umbrsdk.NewFileTokenStore(filepath.Join(dir, "tokens.json")),
		umbrsdk.NewFileDeviceStore(filepath.Join(dir, "device.json")), nil
}

func credentialDir(cfg Config) (string, error) {
	configDir, err := apputils.GetConfigDir()
	if err != nil {
		return "", fmt.Errorf("获取 Umbra 凭据目录失败: %w", err)
	}
	identity := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/") + "\x00" + strings.TrimSpace(cfg.ClientID)
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(configDir, "umbra", hex.EncodeToString(sum[:16])), nil
}
