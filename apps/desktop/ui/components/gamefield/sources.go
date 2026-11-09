package gamefield

import (
	"lunabox/backend/common/enums"
	"lunabox/desktop/ui/metasource"
)

// MetadataSourceTypes 是可以手动关联的元数据来源，顺序同 GameEditPanel.tsx 的 metadataSourceTypes：
// Steam 排在 Ymgal 后面，其余按 ALL_METADATA_SOURCES（编辑标签页的“添加来源”与添加游戏的手动填写共用）。
var MetadataSourceTypes = metasource.Ordered(enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam)
