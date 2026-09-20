//go:build windows

package umbra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"golang.org/x/sys/windows"
	"lunabox/internal/utils/apputils"
)

const cryptProtectUIForbidden = 0x1

var protectedFileLocks sync.Map

type protectedFile struct {
	path string
	mu   *sync.Mutex
}

type tokenStore struct{ file *protectedFile }
type deviceStore struct{ file *protectedFile }

func newCredentialStores(cfg Config) (umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	dir, err := credentialDir(cfg)
	if err != nil {
		return nil, nil, err
	}
	return &tokenStore{file: newProtectedFile(filepath.Join(dir, "tokens.bin"))},
		&deviceStore{file: newProtectedFile(filepath.Join(dir, "device.bin"))}, nil
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

func newProtectedFile(path string) *protectedFile {
	lock, _ := protectedFileLocks.LoadOrStore(path, &sync.Mutex{})
	return &protectedFile{path: path, mu: lock.(*sync.Mutex)}
}

func (s *protectedFile) load(ctx context.Context, target any) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	encrypted, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	plain, err := unprotectData(encrypted)
	if err != nil {
		return false, fmt.Errorf("解密 Umbra 凭据失败: %w", err)
	}
	if err := json.Unmarshal(plain, target); err != nil {
		return false, fmt.Errorf("解析 Umbra 凭据失败: %w", err)
	}
	return true, nil
}

func (s *protectedFile) save(ctx context.Context, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plain, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encrypted, err := protectData(plain)
	if err != nil {
		return fmt.Errorf("加密 Umbra 凭据失败: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, encrypted, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *protectedFile) clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *tokenStore) Load(ctx context.Context) (*umbrsdk.TokenSet, error) {
	var token umbrsdk.TokenSet
	found, err := s.file.load(ctx, &token)
	if err != nil || !found {
		return nil, err
	}
	return &token, nil
}

func (s *tokenStore) Save(ctx context.Context, token *umbrsdk.TokenSet) error {
	return s.file.save(ctx, token)
}

func (s *tokenStore) Clear(ctx context.Context) error { return s.file.clear(ctx) }

func (s *deviceStore) Load(ctx context.Context) (*umbrsdk.DeviceCredentials, error) {
	var credentials umbrsdk.DeviceCredentials
	found, err := s.file.load(ctx, &credentials)
	if err != nil || !found {
		return nil, err
	}
	return &credentials, nil
}

func (s *deviceStore) Save(ctx context.Context, credentials *umbrsdk.DeviceCredentials) error {
	return s.file.save(ctx, credentials)
}

func (s *deviceStore) Clear(ctx context.Context) error { return s.file.clear(ctx) }

func protectData(data []byte) ([]byte, error) {
	return transformData(data, windows.CryptProtectData)
}

func unprotectData(data []byte) ([]byte, error) {
	return transformData(data, func(in *windows.DataBlob, _ *uint16, entropy *windows.DataBlob, reserved uintptr, prompt *windows.CryptProtectPromptStruct, flags uint32, out *windows.DataBlob) error {
		return windows.CryptUnprotectData(in, nil, entropy, reserved, prompt, flags, out)
	})
}

type cryptTransform func(*windows.DataBlob, *uint16, *windows.DataBlob, uintptr, *windows.CryptProtectPromptStruct, uint32, *windows.DataBlob) error

func transformData(data []byte, transform cryptTransform) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("待处理数据为空")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := transform(&in, nil, nil, 0, nil, cryptProtectUIForbidden, &out); err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}
