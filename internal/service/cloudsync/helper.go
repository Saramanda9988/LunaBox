package cloudsync

import (
	"context"
	"database/sql"
	"lunabox/internal/appconf"
	"lunabox/internal/common/dto"
)

type Snapshot = dto.CloudSyncSnapshot
type Game = dto.CloudSyncGame
type Category = dto.CloudSyncCategory
type Relation = dto.CloudSyncRelation
type PlaySession = dto.CloudSyncPlaySession
type GameProgress = dto.CloudSyncGameProgress
type GameTag = dto.CloudSyncGameTag
type CoverAsset = dto.CloudSyncCoverAsset
type LocalCover = dto.CloudSyncLocalCover
type LocalState = dto.CloudSyncLocalState
type Tombstone = dto.CloudSyncTombstone
type Candidate = dto.CloudSyncCandidate

const (
	SchemaVersion = 1
	SnapshotKey   = "sync/library/latest.json"
	LibraryDir    = "sync/library"
	CoverDir      = "sync/covers"

	entityGame         = "game"
	entityCategory     = "category"
	entityGameCategory = "game_category"
	entityPlaySession  = "play_session"
	entityGameProgress = "game_progress"
	entityGameTag      = "game_tag"

	systemFavoritesCategoryID = "system:favorites"
)

type Helper struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewHelper(ctx context.Context, db *sql.DB, config *appconf.AppConfig) *Helper {
	return &Helper{
		ctx:    ctx,
		db:     db,
		config: config,
	}
}

func relationTombstoneID(gameID, categoryID string) string {
	return gameID + "::" + categoryID
}

func tagTombstoneID(gameID, source, name string) string {
	return gameID + "::" + source + "::" + name
}
