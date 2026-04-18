package cloudsync

import (
	"lunabox/internal/common/enums"
	"lunabox/internal/models"
	"path/filepath"
	"strings"
)

func gameFromModel(game models.Game) Game {
	return Game{
		ID:          game.ID,
		Name:        game.Name,
		Company:     game.Company,
		Summary:     game.Summary,
		Rating:      game.Rating,
		ReleaseDate: game.ReleaseDate,
		Status:      string(game.Status),
		SourceType:  string(game.SourceType),
		SourceID:    game.SourceID,
		CreatedAt:   game.CreatedAt,
		UpdatedAt:   game.UpdatedAt,
	}
}

func gameToModel(game Game, coverURL string) models.Game {
	return models.Game{
		ID:          game.ID,
		Name:        game.Name,
		CoverURL:    coverURL,
		Company:     game.Company,
		Summary:     game.Summary,
		Rating:      game.Rating,
		ReleaseDate: game.ReleaseDate,
		Status:      enums.GameStatus(game.Status),
		SourceType:  enums.SourceType(game.SourceType),
		SourceID:    game.SourceID,
		CreatedAt:   game.CreatedAt,
		UpdatedAt:   game.UpdatedAt,
	}
}

func categoryFromModel(category models.Category) Category {
	return Category{
		ID:        category.ID,
		Name:      category.Name,
		Emoji:     category.Emoji,
		IsSystem:  category.IsSystem,
		CreatedAt: category.CreatedAt,
		UpdatedAt: category.UpdatedAt,
	}
}

func categoryToModel(category Category) models.Category {
	return models.Category{
		ID:        category.ID,
		Name:      category.Name,
		Emoji:     category.Emoji,
		IsSystem:  category.IsSystem,
		CreatedAt: category.CreatedAt,
		UpdatedAt: category.UpdatedAt,
	}
}

func relationFromModel(relation models.GameCategory) Relation {
	return Relation{
		GameID:     relation.GameID,
		CategoryID: relation.CategoryID,
		UpdatedAt:  relation.UpdatedAt,
	}
}

func relationToModel(relation Relation) models.GameCategory {
	return models.GameCategory{
		GameID:     relation.GameID,
		CategoryID: relation.CategoryID,
		UpdatedAt:  relation.UpdatedAt,
	}
}

func playSessionFromModel(session models.PlaySession) PlaySession {
	return PlaySession{
		ID:        session.ID,
		GameID:    session.GameID,
		StartTime: session.StartTime,
		EndTime:   session.EndTime,
		Duration:  session.Duration,
		UpdatedAt: session.UpdatedAt,
	}
}

func playSessionToModel(session PlaySession) models.PlaySession {
	return models.PlaySession{
		ID:        session.ID,
		GameID:    session.GameID,
		StartTime: session.StartTime,
		EndTime:   session.EndTime,
		Duration:  session.Duration,
		UpdatedAt: session.UpdatedAt,
	}
}

func gameProgressFromModel(progress models.GameProgress) GameProgress {
	return GameProgress{
		ID:              progress.ID,
		GameID:          progress.GameID,
		Chapter:         progress.Chapter,
		Route:           progress.Route,
		ProgressNote:    progress.ProgressNote,
		SpoilerBoundary: progress.SpoilerBoundary,
		UpdatedAt:       progress.UpdatedAt,
	}
}

func gameProgressToModel(progress GameProgress) models.GameProgress {
	return models.GameProgress{
		ID:              progress.ID,
		GameID:          progress.GameID,
		Chapter:         progress.Chapter,
		Route:           progress.Route,
		ProgressNote:    progress.ProgressNote,
		SpoilerBoundary: progress.SpoilerBoundary,
		UpdatedAt:       progress.UpdatedAt,
	}
}

func gameTagFromModel(tag models.GameTag) GameTag {
	return GameTag{
		ID:        tag.ID,
		GameID:    tag.GameID,
		Name:      tag.Name,
		Source:    tag.Source,
		Weight:    tag.Weight,
		IsSpoiler: tag.IsSpoiler,
		CreatedAt: tag.CreatedAt,
		UpdatedAt: tag.UpdatedAt,
	}
}

func gameTagToModel(tag GameTag) models.GameTag {
	return models.GameTag{
		ID:        tag.ID,
		GameID:    tag.GameID,
		Name:      tag.Name,
		Source:    tag.Source,
		Weight:    tag.Weight,
		IsSpoiler: tag.IsSpoiler,
		CreatedAt: tag.CreatedAt,
		UpdatedAt: tag.UpdatedAt,
	}
}

func tombstoneFromModel(tombstone models.SyncTombstone) Tombstone {
	return Tombstone{
		EntityType: tombstone.EntityType,
		EntityID:   tombstone.EntityID,
		DeletedAt:  tombstone.DeletedAt,
	}
}

func tombstoneToModel(tombstone Tombstone) models.SyncTombstone {
	return models.SyncTombstone{
		EntityType:  tombstone.EntityType,
		EntityID:    tombstone.EntityID,
		ParentID:    "",
		SecondaryID: "",
		DeletedAt:   tombstone.DeletedAt,
	}
}

func newLocalCover(game models.Game, coverPath, coverURL string) LocalCover {
	return LocalCover{
		Asset: CoverAsset{
			GameID:    game.ID,
			Ext:       strings.ToLower(filepath.Ext(coverPath)),
			UpdatedAt: game.UpdatedAt,
		},
		LocalPath: coverPath,
		LocalURL:  coverURL,
	}
}
