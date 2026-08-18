//go:build !windows && !linux

package umbra

import (
	"fmt"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
)

func newCredentialStores(Config) (umbrsdk.TokenStore, umbrsdk.DeviceStore, error) {
	return nil, nil, fmt.Errorf("Umbra 凭据存储仅支持 Windows")
}

func installIDPath() (string, error) {
	return "", fmt.Errorf("Umbra 设备注册仅支持 Windows")
}
