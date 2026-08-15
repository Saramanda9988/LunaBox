package dto

import "time"

// CloudSyncManifest 是 v2 远端布局的入口文件。
// 同步成功后写入 sync/library/manifest.json，是远端权威状态的唯一索引。
// 不在 manifest 引用集合中的桶/单文件视为 orphan，可被安全清理。
type CloudSyncManifest struct {
	SchemaVersion int                                      `json:"schema_version"`
	RevisionID    string                                   `json:"revision_id"`
	ExportedAt    time.Time                                `json:"exported_at"`
	DeviceID      string                                   `json:"device_id"`
	Buckets       map[string]map[string]CloudSyncBucketRef `json:"buckets"`
	Singletons    map[string]CloudSyncBucketRef            `json:"singletons"`
	Covers        []CloudSyncCoverRef                      `json:"covers"`
}

// CloudSyncBucketRef 描述一个桶文件或单文件的指纹。
// hash 由桶内 items 按主键排序后做 canonical JSON marshal 再 sha256 截断 32 字符得到。
type CloudSyncBucketRef struct {
	Hash      string    `json:"hash"`
	Count     int       `json:"count"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CloudSyncBucketFile 是单个桶文件的完整内容。
// items 元素类型对应 BucketKey 前缀，与现有 CloudSyncGame / CloudSyncPlaySession / ... 一致。
// 反序列化时按 EntityType 字段决定具体类型。
type CloudSyncBucketFile struct {
	SchemaVersion int    `json:"schema_version"`
	BucketKey     string `json:"bucket_key"`
	// Items 字段在不同实体类型下使用具体的具名字段以保持类型安全
	Games           []CloudSyncGame               `json:"games,omitempty"`
	PlaySessions    []CloudSyncPlaySession        `json:"play_sessions,omitempty"`
	GameProgresses  []CloudSyncGameProgress       `json:"game_progresses,omitempty"`
	GameReviews     []CloudSyncGameReview         `json:"game_reviews,omitempty"`
	GameTags        []CloudSyncGameTag            `json:"game_tags,omitempty"`
	MetadataSources []CloudSyncGameMetadataSource `json:"game_metadata_sources,omitempty"`
	GameCategories  []CloudSyncRelation           `json:"game_categories,omitempty"`
	Categories      []CloudSyncCategory           `json:"categories,omitempty"`
	Tombstones      []CloudSyncTombstone          `json:"tombstones,omitempty"`
}

// CloudSyncCoverRef 是 manifest 中对一个封面文件的引用。
// 复用现有 CoverAsset 的 game_id + ext + updated_at；hash 用于跨设备一致性判断。
type CloudSyncCoverRef struct {
	GameID    string    `json:"game_id"`
	Ext       string    `json:"ext"`
	UpdatedAt time.Time `json:"updated_at"`
	Hash      string    `json:"hash,omitempty"`
}
