package gamehelper

import (
	"fmt"
	"slices"
	"strings"

	"lunabox/backend/common/enums"
	"lunabox/backend/models"
)

func IsSupportedMetadataSource(source enums.SourceType) bool {
	return slices.Contains(enums.MetadataSourceTypes, NormalizeMetadataSourceType(source))
}

func NormalizeMetadataSource(source enums.SourceType, sourceID string) (enums.SourceType, string, error) {
	source = NormalizeMetadataSourceType(source)
	sourceID = strings.TrimSpace(sourceID)
	if sourceID == "" {
		return "", "", fmt.Errorf("元数据来源 ID 不能为空")
	}
	if !IsSupportedMetadataSource(source) {
		return "", "", fmt.Errorf("不支持的元数据来源: %s", source)
	}
	return source, sourceID, nil
}

func ValidateInitialMetadataSources(sources []models.GameMetadataSource) error {
	seen := make(map[enums.SourceType]struct{}, len(sources))
	for _, item := range sources {
		source, _, err := NormalizeMetadataSource(item.SourceType, item.SourceID)
		if err != nil {
			return err
		}
		if _, exists := seen[source]; exists {
			return fmt.Errorf("同一游戏的 %s 元数据记录存在多个，请移除错误的候选项", source)
		}
		seen[source] = struct{}{}
	}
	return nil
}

func DefaultMetadataSourceValue(source enums.SourceType) string {
	if source == "" {
		return string(enums.Local)
	}
	return string(source)
}

func NormalizeDefaultMetadataSource(source enums.SourceType, sourceID string) (enums.SourceType, string) {
	source = NormalizeMetadataSourceType(source)
	sourceID = strings.TrimSpace(sourceID)
	if source == "" || source == enums.Local {
		return enums.Local, ""
	}
	return source, sourceID
}
