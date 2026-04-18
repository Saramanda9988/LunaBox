package cloudsync

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

func (h *Helper) MergeSnapshots(local, remote Snapshot, remoteExists bool) Snapshot {
	if !remoteExists {
		local.RevisionID = uuid.New().String()
		local.ExportedAt = time.Now()
		local.DeviceID = h.currentDeviceID()
		sortSnapshot(&local)
		return local
	}

	merged := Snapshot{
		SchemaVersion: SchemaVersion,
		RevisionID:    uuid.New().String(),
		ExportedAt:    time.Now(),
		DeviceID:      h.currentDeviceID(),
	}

	localGameMap := mapGames(local.Games)
	remoteGameMap := mapGames(remote.Games)
	localGameTombstones := mapTombstones(local.Tombstones, entityGame)
	remoteGameTombstones := mapTombstones(remote.Tombstones, entityGame)
	for _, id := range unionKeys4(localGameMap, remoteGameMap, localGameTombstones, remoteGameTombstones) {
		if game, ok, deletedAt := mergeGame(localGameMap[id], remoteGameMap[id], localGameTombstones[id], remoteGameTombstones[id]); ok {
			merged.Games = append(merged.Games, game)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityGame, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localCategoryMap := mapCategories(local.Categories)
	remoteCategoryMap := mapCategories(remote.Categories)
	localCategoryTombstones := mapTombstones(local.Tombstones, entityCategory)
	remoteCategoryTombstones := mapTombstones(remote.Tombstones, entityCategory)
	for _, id := range unionKeys4(localCategoryMap, remoteCategoryMap, localCategoryTombstones, remoteCategoryTombstones) {
		if category, ok, deletedAt := mergeCategory(localCategoryMap[id], remoteCategoryMap[id], localCategoryTombstones[id], remoteCategoryTombstones[id]); ok {
			merged.Categories = append(merged.Categories, category)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityCategory, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localRelationMap := mapRelations(local.GameCategories)
	remoteRelationMap := mapRelations(remote.GameCategories)
	localRelationTombstones := mapTombstones(local.Tombstones, entityGameCategory)
	remoteRelationTombstones := mapTombstones(remote.Tombstones, entityGameCategory)
	for _, id := range unionKeys4(localRelationMap, remoteRelationMap, localRelationTombstones, remoteRelationTombstones) {
		if relation, ok, deletedAt := mergeRelation(localRelationMap[id], remoteRelationMap[id], localRelationTombstones[id], remoteRelationTombstones[id]); ok {
			merged.GameCategories = append(merged.GameCategories, relation)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityGameCategory, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localSessionMap := mapPlaySessions(local.PlaySessions)
	remoteSessionMap := mapPlaySessions(remote.PlaySessions)
	localSessionTombstones := mapTombstones(local.Tombstones, entityPlaySession)
	remoteSessionTombstones := mapTombstones(remote.Tombstones, entityPlaySession)
	for _, id := range unionKeys4(localSessionMap, remoteSessionMap, localSessionTombstones, remoteSessionTombstones) {
		if session, ok, deletedAt := mergePlaySession(localSessionMap[id], remoteSessionMap[id], localSessionTombstones[id], remoteSessionTombstones[id]); ok {
			merged.PlaySessions = append(merged.PlaySessions, session)
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityPlaySession, EntityID: id, DeletedAt: deletedAt})
		}
	}

	mergedGameMap := mapGames(merged.Games)

	localProgressMap := mapGameProgresses(local.GameProgresses)
	remoteProgressMap := mapGameProgresses(remote.GameProgresses)
	localProgressTombstones := mapTombstones(local.Tombstones, entityGameProgress)
	remoteProgressTombstones := mapTombstones(remote.Tombstones, entityGameProgress)
	for _, id := range unionKeys4(localProgressMap, remoteProgressMap, localProgressTombstones, remoteProgressTombstones) {
		if progress, ok, deletedAt := mergeGameProgress(localProgressMap[id], remoteProgressMap[id], localProgressTombstones[id], remoteProgressTombstones[id]); ok {
			if _, gameExists := mergedGameMap[progress.GameID]; gameExists {
				merged.GameProgresses = append(merged.GameProgresses, progress)
			}
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityGameProgress, EntityID: id, DeletedAt: deletedAt})
		}
	}

	localTagMap := mapGameTags(local.GameTags)
	remoteTagMap := mapGameTags(remote.GameTags)
	localTagTombstones := mapTombstones(local.Tombstones, entityGameTag)
	remoteTagTombstones := mapTombstones(remote.Tombstones, entityGameTag)
	for _, id := range unionKeys4(localTagMap, remoteTagMap, localTagTombstones, remoteTagTombstones) {
		if tag, ok, deletedAt := mergeGameTag(localTagMap[id], remoteTagMap[id], localTagTombstones[id], remoteTagTombstones[id]); ok {
			if _, gameExists := mergedGameMap[tag.GameID]; gameExists {
				merged.GameTags = append(merged.GameTags, tag)
			}
		} else if !deletedAt.IsZero() {
			merged.Tombstones = append(merged.Tombstones, Tombstone{EntityType: entityGameTag, EntityID: id, DeletedAt: deletedAt})
		}
	}

	merged.Covers = h.mergeCovers(local, remote, merged.Games)
	sortSnapshot(&merged)
	return merged
}

func (h *Helper) mergeCovers(local, remote Snapshot, mergedGames []Game) []CoverAsset {
	localCovers := mapCoverAssets(local.Covers)
	remoteCovers := mapCoverAssets(remote.Covers)
	localGames := mapGames(local.Games)
	remoteGames := mapGames(remote.Games)

	merged := make([]CoverAsset, 0, len(mergedGames))
	for _, game := range mergedGames {
		localGame, hasLocalGame := localGames[game.ID]
		remoteGame, hasRemoteGame := remoteGames[game.ID]
		localCover, hasLocalCover := localCovers[game.ID]
		remoteCover, hasRemoteCover := remoteCovers[game.ID]

		best := Candidate{}
		hasBest := false
		bestDeleted := false
		chosen := CoverAsset{}

		if hasLocalCover {
			best = Candidate{Timestamp: localCover.UpdatedAt, Source: 0}
			chosen = localCover
			hasBest = true
		}
		if hasRemoteCover {
			candidate := Candidate{Timestamp: remoteCover.UpdatedAt, Source: 1}
			if !hasBest || compareCandidate(candidate, best) > 0 {
				best = candidate
				chosen = remoteCover
				hasBest = true
				bestDeleted = false
			}
		}
		if hasLocalGame && !hasLocalCover {
			candidate := Candidate{Timestamp: localGame.UpdatedAt, Source: 0, Deleted: true}
			if !hasBest || compareCandidate(candidate, best) > 0 {
				best = candidate
				hasBest = true
				bestDeleted = true
			}
		}
		if hasRemoteGame && !hasRemoteCover {
			candidate := Candidate{Timestamp: remoteGame.UpdatedAt, Source: 1, Deleted: true}
			if !hasBest || compareCandidate(candidate, best) > 0 {
				best = candidate
				hasBest = true
				bestDeleted = true
			}
		}

		if hasBest && !bestDeleted {
			merged = append(merged, chosen)
		}
	}

	return merged
}

func sortSnapshot(snapshot *Snapshot) {
	sort.Slice(snapshot.Games, func(i, j int) bool { return snapshot.Games[i].ID < snapshot.Games[j].ID })
	sort.Slice(snapshot.Categories, func(i, j int) bool { return snapshot.Categories[i].ID < snapshot.Categories[j].ID })
	sort.Slice(snapshot.GameCategories, func(i, j int) bool {
		left := snapshot.GameCategories[i].GameID + "::" + snapshot.GameCategories[i].CategoryID
		right := snapshot.GameCategories[j].GameID + "::" + snapshot.GameCategories[j].CategoryID
		return left < right
	})
	sort.Slice(snapshot.PlaySessions, func(i, j int) bool { return snapshot.PlaySessions[i].ID < snapshot.PlaySessions[j].ID })
	sort.Slice(snapshot.GameProgresses, func(i, j int) bool { return snapshot.GameProgresses[i].ID < snapshot.GameProgresses[j].ID })
	sort.Slice(snapshot.GameTags, func(i, j int) bool {
		return tagTombstoneID(snapshot.GameTags[i].GameID, snapshot.GameTags[i].Source, snapshot.GameTags[i].Name) <
			tagTombstoneID(snapshot.GameTags[j].GameID, snapshot.GameTags[j].Source, snapshot.GameTags[j].Name)
	})
	sort.Slice(snapshot.Tombstones, func(i, j int) bool {
		left := snapshot.Tombstones[i].EntityType + "::" + snapshot.Tombstones[i].EntityID
		right := snapshot.Tombstones[j].EntityType + "::" + snapshot.Tombstones[j].EntityID
		return left < right
	})
	sort.Slice(snapshot.Covers, func(i, j int) bool { return snapshot.Covers[i].GameID < snapshot.Covers[j].GameID })
}

func mapGames(items []Game) map[string]Game {
	result := make(map[string]Game, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapCategories(items []Category) map[string]Category {
	result := make(map[string]Category, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapRelations(items []Relation) map[string]Relation {
	result := make(map[string]Relation, len(items))
	for _, item := range items {
		result[relationTombstoneID(item.GameID, item.CategoryID)] = item
	}
	return result
}

func mapPlaySessions(items []PlaySession) map[string]PlaySession {
	result := make(map[string]PlaySession, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapGameProgresses(items []GameProgress) map[string]GameProgress {
	result := make(map[string]GameProgress, len(items))
	for _, item := range items {
		result[item.ID] = item
	}
	return result
}

func mapGameTags(items []GameTag) map[string]GameTag {
	result := make(map[string]GameTag, len(items))
	for _, item := range items {
		result[tagTombstoneID(item.GameID, item.Source, item.Name)] = item
	}
	return result
}

func mapCoverAssets(items []CoverAsset) map[string]CoverAsset {
	result := make(map[string]CoverAsset, len(items))
	for _, item := range items {
		result[item.GameID] = item
	}
	return result
}

func mapTombstones(items []Tombstone, entityType string) map[string]time.Time {
	result := make(map[string]time.Time)
	for _, item := range items {
		if item.EntityType == entityType {
			result[item.EntityID] = item.DeletedAt
		}
	}
	return result
}

func unionKeys4[T any, U any](left map[string]T, right map[string]T, third map[string]U, fourth map[string]U) []string {
	seen := make(map[string]struct{}, len(left)+len(right)+len(third)+len(fourth))
	for key := range left {
		seen[key] = struct{}{}
	}
	for key := range right {
		seen[key] = struct{}{}
	}
	for key := range third {
		seen[key] = struct{}{}
	}
	for key := range fourth {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compareCandidate(left, right Candidate) int {
	switch {
	case left.Timestamp.After(right.Timestamp):
		return 1
	case right.Timestamp.After(left.Timestamp):
		return -1
	}
	if left.Source != right.Source {
		if left.Source > right.Source {
			return 1
		}
		return -1
	}
	if left.Deleted != right.Deleted {
		if left.Deleted {
			return 1
		}
		return -1
	}
	return 0
}

func mergeGame(local, remote Game, localDeleted, remoteDeleted time.Time) (Game, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := Game{}

	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return Game{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeCategory(local, remote Category, localDeleted, remoteDeleted time.Time) (Category, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := Category{}
	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return Category{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeRelation(local, remote Relation, localDeleted, remoteDeleted time.Time) (Relation, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := Relation{}
	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return Relation{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergePlaySession(local, remote PlaySession, localDeleted, remoteDeleted time.Time) (PlaySession, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := PlaySession{}
	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return PlaySession{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeGameProgress(local, remote GameProgress, localDeleted, remoteDeleted time.Time) (GameProgress, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := GameProgress{}
	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return GameProgress{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}

func mergeGameTag(local, remote GameTag, localDeleted, remoteDeleted time.Time) (GameTag, bool, time.Time) {
	best := Candidate{}
	hasBest := false
	bestDeleted := false
	bestRecord := GameTag{}
	if !local.UpdatedAt.IsZero() {
		best = Candidate{Timestamp: local.UpdatedAt, Source: 0}
		bestRecord = local
		hasBest = true
	}
	if !remote.UpdatedAt.IsZero() {
		candidate := Candidate{Timestamp: remote.UpdatedAt, Source: 1}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			bestRecord = remote
			hasBest = true
			bestDeleted = false
		}
	}
	if !localDeleted.IsZero() {
		candidate := Candidate{Timestamp: localDeleted, Source: 0, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !remoteDeleted.IsZero() {
		candidate := Candidate{Timestamp: remoteDeleted, Source: 1, Deleted: true}
		if !hasBest || compareCandidate(candidate, best) > 0 {
			best = candidate
			hasBest = true
			bestDeleted = true
		}
	}
	if !hasBest || bestDeleted {
		return GameTag{}, false, best.Timestamp
	}
	return bestRecord, true, time.Time{}
}
