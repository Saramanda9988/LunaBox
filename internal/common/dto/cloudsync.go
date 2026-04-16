package dto

import "time"

type CloudSyncSnapshot struct {
	SchemaVersion  int                     `json:"schema_version"`
	RevisionID     string                  `json:"revision_id"`
	ExportedAt     time.Time               `json:"exported_at"`
	DeviceID       string                  `json:"device_id"`
	Games          []CloudSyncGame         `json:"games"`
	Categories     []CloudSyncCategory     `json:"categories"`
	GameCategories []CloudSyncRelation     `json:"game_categories"`
	PlaySessions   []CloudSyncPlaySession  `json:"play_sessions"`
	GameProgresses []CloudSyncGameProgress `json:"game_progresses"`
	GameTags       []CloudSyncGameTag      `json:"game_tags"`
	Tombstones     []CloudSyncTombstone    `json:"tombstones"`
	Covers         []CloudSyncCoverAsset   `json:"covers"`
}

type CloudSyncGame struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Company     string    `json:"company"`
	Summary     string    `json:"summary"`
	Rating      float64   `json:"rating"`
	ReleaseDate string    `json:"release_date"`
	Status      string    `json:"status"`
	SourceType  string    `json:"source_type"`
	SourceID    string    `json:"source_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CloudSyncCategory struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Emoji     string    `json:"emoji"`
	IsSystem  bool      `json:"is_system"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CloudSyncRelation struct {
	GameID     string    `json:"game_id"`
	CategoryID string    `json:"category_id"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CloudSyncPlaySession struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  int       `json:"duration"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CloudSyncGameProgress struct {
	ID              string    `json:"id"`
	GameID          string    `json:"game_id"`
	Chapter         string    `json:"chapter"`
	Route           string    `json:"route"`
	ProgressNote    string    `json:"progress_note"`
	SpoilerBoundary string    `json:"spoiler_boundary"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CloudSyncGameTag struct {
	ID        string    `json:"id"`
	GameID    string    `json:"game_id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	Weight    float64   `json:"weight"`
	IsSpoiler bool      `json:"is_spoiler"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CloudSyncTombstone struct {
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	DeletedAt  time.Time `json:"deleted_at"`
}

type CloudSyncCoverAsset struct {
	GameID    string    `json:"game_id"`
	Ext       string    `json:"ext"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CloudSyncLocalCover struct {
	Asset     CloudSyncCoverAsset
	LocalPath string
	LocalURL  string
}

type CloudSyncLocalState struct {
	Snapshot CloudSyncSnapshot
	Covers   map[string]CloudSyncLocalCover
}

type CloudSyncCandidate struct {
	Timestamp time.Time
	Source    int
	Deleted   bool
}
