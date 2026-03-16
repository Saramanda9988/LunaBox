package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// GenerateUserID 根据备份密码生成用户 ID
func GenerateUserID(password string) string {
	hash := sha256.Sum256([]byte("lunabox-backup:" + password))
	return hex.EncodeToString(hash[:16])
}
