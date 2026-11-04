package gamefield

import "lunabox/backend/common/enums"

// MetadataSourceTypes 是可以手动关联的元数据来源，顺序同 GameEditPanel.tsx 的 metadataSourceTypes
// （编辑标签页的“添加来源”与添加游戏的手动填写共用）。
var MetadataSourceTypes = []enums.SourceType{
	enums.Bangumi, enums.VNDB, enums.Ymgal, enums.Steam, enums.DLsite, enums.TouchGal, enums.Hikarinagi, enums.ErogameScape,
}
