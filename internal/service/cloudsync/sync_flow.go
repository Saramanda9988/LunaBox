package cloudsync

import (
	"errors"
	"fmt"
	"lunabox/internal/applog"
	"lunabox/internal/service/cloudprovider"
	"time"

	"github.com/google/uuid"
)

// SyncToCloud 是 v2 增量同步的入口。
// 流程：
//  1. 从本地 DB 构建完整 snapshot；
//  2. 一次性 EnsureDir 全部远端子目录（OneDrive 路径成本最大化收敛）；
//  3. 读远端 manifest：
//     - 缺失 → 尝试 v1 latest.json，命中则迁移；都缺失则视为首同步全量推；
//     - 存在 → 走 v2 差量主流程；
//  4. 任何一步失败立即返回，**不**更新 cloud_sync_state。
func (h *Helper) SyncToCloud(provider cloudprovider.CloudStorageProvider) error {
	applog.LogInfof(h.ctx, "CloudSync: sync started provider=%T concurrency=%d", provider, ConcurrencyFor(provider))
	localState, err := h.BuildLocalState()
	if err != nil {
		return fmt.Errorf("build local state: %w", err)
	}

	if err := h.EnsureSyncDirs(provider); err != nil {
		return fmt.Errorf("ensure sync dirs: %w", err)
	}

	remoteManifest, manifestExists, err := h.LoadRemoteManifest(provider)
	if err != nil {
		if errors.Is(err, ErrManifestSchemaTooNew) {
			return err
		}
		return fmt.Errorf("load remote manifest: %w", err)
	}

	if !manifestExists {
		return h.bootstrapOrMigrate(provider, localState)
	}
	return h.runIncrementalSync(provider, localState, remoteManifest)
}

// bootstrapOrMigrate 处理"远端无 v2 manifest"的两种子情况：
//   - 远端有 v1 latest.json → 拉下来作为 remote，与本地走一次 LWW merge，
//     再以新布局上传所有桶 + manifest，最后删除 latest.json；
//   - 远端完全空 → 用本地数据生成全套 v2 桶 + manifest 上传。
func (h *Helper) bootstrapOrMigrate(provider cloudprovider.CloudStorageProvider, localState LocalState) error {
	v1Snapshot, v1Exists, err := h.LoadV1Snapshot(provider)
	if err != nil {
		return fmt.Errorf("probe v1 snapshot: %w", err)
	}

	var merged Snapshot
	if v1Exists {
		applog.LogInfof(h.ctx, "CloudSync: migrating v1 latest.json → v2 layout")
		merged = h.MergeSnapshots(localState.Snapshot, v1Snapshot, true)
	} else {
		applog.LogInfof(h.ctx, "CloudSync: remote empty, performing first v2 bootstrap upload")
		merged = h.MergeSnapshots(localState.Snapshot, Snapshot{}, false)
	}

	coverURLs, err := h.ReconcileCoverAssets(provider, localState, v1Snapshot, v1Exists, merged)
	if err != nil {
		return fmt.Errorf("reconcile covers during bootstrap: %w", err)
	}
	if err := h.ApplyMergedSnapshot(merged, coverURLs); err != nil {
		return fmt.Errorf("apply merged snapshot during bootstrap: %w", err)
	}

	mergedBuckets := Bucketize(merged)
	revisionID := uuid.New().String()
	newManifest, err := BuildManifestFromBuckets(mergedBuckets, merged.Categories, merged.Tombstones, merged.Covers, h.currentDeviceID(), revisionID, h.now())
	if err != nil {
		return fmt.Errorf("build manifest during bootstrap: %w", err)
	}

	// 全部桶 + 两个 singleton 可合并走 batch；manifest 仍最后单独上传。
	allBucketKeys := allBucketKeysFromManifest(newManifest)
	if err := h.SaveRemoteLibraryFiles(
		provider,
		mergedBuckets,
		allBucketKeys,
		merged.Categories,
		merged.Tombstones,
		[]string{SingletonCategories, SingletonTombstones},
	); err != nil {
		return fmt.Errorf("upload library files during bootstrap: %w", err)
	}
	if err := h.SaveRemoteManifest(provider, newManifest); err != nil {
		return fmt.Errorf("upload manifest during bootstrap: %w", err)
	}

	// manifest 成功后再删除 v1 latest.json：把"v2 已成型"作为唯一切换点
	if v1Exists {
		if err := h.DeleteV1Snapshot(provider); err != nil {
			applog.LogWarningf(h.ctx, "CloudSync: bootstrap finished but v1 latest.json delete failed: %v", err)
		}
	}

	return h.persistSyncState(mergedBuckets, merged.Categories, merged.Tombstones, newManifest)
}

// runIncrementalSync 是 v2 主流程：根据 hash diff 决定拉/推哪些桶，
// 合并落库，最后写 manifest 与 cloud_sync_state。
func (h *Helper) runIncrementalSync(provider cloudprovider.CloudStorageProvider, localState LocalState, remoteManifest Manifest) error {
	localBuckets := Bucketize(localState.Snapshot)
	localManifest, err := BuildManifestFromBuckets(
		localBuckets,
		localState.Snapshot.Categories,
		localState.Snapshot.Tombstones,
		localState.Snapshot.Covers,
		h.currentDeviceID(),
		"", // 本地视图不分配 revision，最终上传时再生成
		h.now(),
	)
	if err != nil {
		return fmt.Errorf("build local manifest: %w", err)
	}

	cachedState, err := LoadSyncState(h.ctx, h.db)
	if err != nil {
		return fmt.Errorf("load cloud_sync_state: %w", err)
	}

	diff := DiffBuckets(localManifest, cachedState, remoteManifest, true)
	if !diff.HasWork() {
		applog.LogInfof(h.ctx, "CloudSync: nothing to do (local and remote both stable)")
		// 仍然 persist 一次 state，把 manifest revision_id 写入 _manifest 行，便于后续追踪
		return h.persistSyncState(localBuckets, localState.Snapshot.Categories, localState.Snapshot.Tombstones, remoteManifest)
	}

	// 封面引用独立存放在 manifest 中，但其 LWW 语义依赖对应游戏的 updated_at。
	// 因此封面不一致时也要把远端游戏桶拉入本次 merge。
	toPull := append([]string(nil), diff.ToPull...)
	for _, gameID := range diff.CoversChanged {
		ch := BucketKeyOfGame(gameID)
		if remoteManifest.Buckets[EntityKeyGames][ch].Count > 0 {
			toPull = appendUniqueString(toPull, BucketKey(EntityKeyGames, ch))
		}
	}
	for _, key := range append(append([]string(nil), diff.ToPull...), diff.LocalChanged...) {
		entity, ch, ok := splitBucketKey(key)
		if !ok || (entity != EntityKeyGameMetadataSources && entity != EntityKeyGameReviews) {
			continue
		}
		if remoteManifest.Buckets[EntityKeyGames][ch].Count > 0 {
			toPull = appendUniqueString(toPull, BucketKey(EntityKeyGames, ch))
		}
	}

	// 拉差异桶
	remoteBuckets, err := h.LoadRemoteBuckets(provider, toPull)
	if err != nil {
		return fmt.Errorf("load remote buckets: %w", err)
	}

	// 拉差异 singletons —— 即使本地端有变化，也要拉远端做 LWW
	var remoteCategories []Category
	var remoteTombstones []Tombstone
	if len(diff.SingletonsToPull) > 0 {
		cats, tombs, _, sErr := h.LoadRemoteSingletons(provider, diff.SingletonsToPull)
		if sErr != nil {
			return fmt.Errorf("load remote singletons: %w", sErr)
		}
		remoteCategories = cats
		remoteTombstones = tombs
	}

	// 构造 partial snapshot 喂给 MergeSnapshots
	// changedBuckets = union(ToPull, LocalChanged) —— 涵盖任意一侧有变化的桶
	changed := unionBucketKeys(toPull, diff.LocalChanged)
	localSubset, remoteSubset := buildMergeSubsets(localBuckets, remoteBuckets, changed,
		localState.Snapshot.Categories, remoteCategories,
		localState.Snapshot.Tombstones, remoteTombstones,
		diff.SingletonsToPull, diff.SingletonsChanged,
	)
	localSubset.Covers = localState.Snapshot.Covers
	remoteSubset.Covers = remoteManifestToSnapshot(remoteManifest).Covers

	mergedSubset := h.MergeSnapshots(localSubset, remoteSubset, true)
	for _, source := range mergedSubset.MetadataSources {
		changed[BucketKey(EntityKeyGameMetadataSources, BucketKeyOfGame(source.GameID))] = struct{}{}
	}
	for _, review := range mergedSubset.GameReviews {
		changed[BucketKey(EntityKeyGameReviews, BucketKeyOfGame(review.GameID))] = struct{}{}
	}

	// 拼回 unchanged buckets：未变化桶的本地数据本身就等于远端，直接复用
	finalSnapshot := assembleFinalSnapshot(localBuckets, remoteBuckets, changed, mergedSubset, localState.Snapshot)

	coverURLs, err := h.ReconcileCoverAssets(provider, localState, remoteManifestToSnapshot(remoteManifest), true, finalSnapshot)
	if err != nil {
		return fmt.Errorf("reconcile covers: %w", err)
	}
	if err := h.ApplyMergedSnapshot(finalSnapshot, coverURLs); err != nil {
		return fmt.Errorf("apply merged snapshot: %w", err)
	}

	// 重新分桶（merge 后内容已变化）
	finalBuckets := Bucketize(finalSnapshot)
	revisionID := uuid.New().String()
	finalManifest, err := BuildManifestFromBuckets(
		finalBuckets,
		finalSnapshot.Categories,
		finalSnapshot.Tombstones,
		finalSnapshot.Covers,
		h.currentDeviceID(),
		revisionID,
		h.now(),
	)
	if err != nil {
		return fmt.Errorf("build final manifest: %w", err)
	}

	// 决定要上传哪些：与远端 manifest 比对 hash
	toPushBuckets := pushBucketKeys(finalManifest, remoteManifest)
	toPushSingletons := pushSingletonNames(finalManifest, remoteManifest)

	if err := h.SaveRemoteLibraryFiles(
		provider,
		finalBuckets,
		toPushBuckets,
		finalSnapshot.Categories,
		finalSnapshot.Tombstones,
		toPushSingletons,
	); err != nil {
		return fmt.Errorf("upload library files: %w", err)
	}

	// 总是写 manifest（即便桶/单文件没变也要写，覆盖远端 revision_id 推进）
	if err := h.SaveRemoteManifest(provider, finalManifest); err != nil {
		return fmt.Errorf("upload manifest: %w", err)
	}

	return h.persistSyncState(finalBuckets, finalSnapshot.Categories, finalSnapshot.Tombstones, finalManifest)
}

// persistSyncState 把每个桶/单文件的 hash 与 manifest revision 写回 cloud_sync_state。
// 仅在 SyncNow 完整成功后调用；事务内 upsert 保证 "全部成功才更新"。
func (h *Helper) persistSyncState(
	buckets map[string]map[string]*BucketContent,
	categories []Category,
	tombstones []Tombstone,
	manifest Manifest,
) error {
	rows := make([]SyncStateRow, 0, len(EntityKeys())*BucketCount+3)
	now := h.now()

	for _, entityKey := range EntityKeys() {
		byBucket := buckets[entityKey]
		for _, ch := range bucketKeysSorted() {
			bc := byBucket[ch]
			hash, err := BucketHashOf(entityKey, bc)
			if err != nil {
				return fmt.Errorf("hash bucket %s/%s for state: %w", entityKey, ch, err)
			}
			rows = append(rows, SyncStateRow{
				BucketKey:        BucketKey(entityKey, ch),
				LocalHash:        hash,
				RemoteHash:       manifest.Buckets[entityKey][ch].Hash,
				RemoteRevisionID: manifest.RevisionID,
				UpdatedAt:        now,
			})
		}
	}

	catHash, err := BucketHash(categories)
	if err != nil {
		return fmt.Errorf("hash categories for state: %w", err)
	}
	rows = append(rows, SyncStateRow{
		BucketKey:        SingletonStateKey(SingletonCategories),
		LocalHash:        catHash,
		RemoteHash:       manifest.Singletons[SingletonCategories].Hash,
		RemoteRevisionID: manifest.RevisionID,
		UpdatedAt:        now,
	})
	tsHash, err := BucketHash(tombstones)
	if err != nil {
		return fmt.Errorf("hash tombstones for state: %w", err)
	}
	rows = append(rows, SyncStateRow{
		BucketKey:        SingletonStateKey(SingletonTombstones),
		LocalHash:        tsHash,
		RemoteHash:       manifest.Singletons[SingletonTombstones].Hash,
		RemoteRevisionID: manifest.RevisionID,
		UpdatedAt:        now,
	})

	rows = append(rows, SyncStateRow{
		BucketKey:        StateKeyManifest,
		LocalHash:        manifest.RevisionID,
		RemoteHash:       manifest.RevisionID,
		RemoteRevisionID: manifest.RevisionID,
		UpdatedAt:        now,
	})

	return SaveSyncState(h.ctx, h.db, rows)
}

// allBucketKeysFromManifest 列出 manifest 中全部非空桶的 key（"games/3" 等）。
// 用于 bootstrap 上传：首同步必须把所有非空桶推上去。
func allBucketKeysFromManifest(m Manifest) []string {
	keys := make([]string, 0, len(EntityKeys())*BucketCount)
	for _, entityKey := range EntityKeys() {
		for _, ch := range bucketKeysSorted() {
			ref := m.Buckets[entityKey][ch]
			if ref.Count > 0 {
				keys = append(keys, BucketKey(entityKey, ch))
			}
		}
	}
	return keys
}

// unionBucketKeys 去重合并两个桶 key 列表。
func unionBucketKeys(a, b []string) map[string]struct{} {
	out := make(map[string]struct{}, len(a)+len(b))
	for _, k := range a {
		out[k] = struct{}{}
	}
	for _, k := range b {
		out[k] = struct{}{}
	}
	return out
}

// buildMergeSubsets 把"涉及合并"的桶里的 items 拼成两个 partial Snapshot 喂给 MergeSnapshots。
// tombstones 必须**完整**传入，否则跨桶引用的删除墓碑会丢失（破坏 merge 语义）。
// categories 同理：merge 阶段不应当因为某条 category 落在未拉远端的桶里而误删。
func buildMergeSubsets(
	localBuckets, remoteBuckets map[string]map[string]*BucketContent,
	changed map[string]struct{},
	localCategories, remoteCategories []Category,
	localTombstones, remoteTombstones []Tombstone,
	singletonsToPull, singletonsChanged []string,
) (Snapshot, Snapshot) {
	local := Snapshot{Tombstones: localTombstones, Categories: localCategories}
	remote := Snapshot{Tombstones: remoteTombstones, Categories: remoteCategories}

	// 远端如果没拉过 singleton（说明远端 hash 与缓存一致），按"远端 = 本地"等价处理
	if !containsString(singletonsToPull, SingletonCategories) {
		remote.Categories = localCategories
	}
	if !containsString(singletonsToPull, SingletonTombstones) {
		remote.Tombstones = localTombstones
	}
	// 本地 singleton 没变时同理
	if !containsString(singletonsChanged, SingletonCategories) && !containsString(singletonsToPull, SingletonCategories) {
		local.Categories = localCategories // already that
	}

	for key := range changed {
		entity, ch, ok := splitBucketKey(key)
		if !ok {
			continue
		}
		if bc := localBuckets[entity][ch]; bc != nil {
			appendBucketIntoSnapshot(&local, entity, bc)
		}
		if rbc, ok := remoteBuckets[entity][ch]; ok && rbc != nil {
			appendBucketIntoSnapshot(&remote, entity, rbc)
		}
	}
	return local, remote
}

// assembleFinalSnapshot 把 merge 的结果与未变化桶的本地数据拼成一个完整 snapshot，喂给 ApplyMergedSnapshot。
// finalCategories / finalTombstones 由 mergedSubset 决定（merge 已经处理了 LWW）。
func assembleFinalSnapshot(
	localBuckets, remoteBuckets map[string]map[string]*BucketContent,
	changed map[string]struct{},
	mergedSubset Snapshot,
	originalLocal Snapshot,
) Snapshot {
	out := Snapshot{
		SchemaVersion: SchemaVersion,
		Categories:    mergedSubset.Categories,
		Tombstones:    mergedSubset.Tombstones,
	}
	for _, cover := range originalLocal.Covers {
		gameBucket := BucketKey(EntityKeyGames, BucketKeyOfGame(cover.GameID))
		if _, isChanged := changed[gameBucket]; !isChanged {
			out.Covers = append(out.Covers, cover)
		}
	}
	// 受影响游戏桶的封面必须使用 LWW merge 的结果，不能回退到本地快照。
	out.Covers = append(out.Covers, mergedSubset.Covers...)

	// 从 mergedSubset 拿到 changed buckets 的合并结果
	mergedByID := indexSnapshotByGameBucket(mergedSubset)

	for _, entityKey := range EntityKeys() {
		for _, ch := range bucketKeysSorted() {
			key := BucketKey(entityKey, ch)
			if _, isChanged := changed[key]; isChanged {
				switch entityKey {
				case EntityKeyGames:
					out.Games = append(out.Games, mergedByID[EntityKeyGames][ch].Games...)
				case EntityKeyPlaySessions:
					out.PlaySessions = append(out.PlaySessions, mergedByID[EntityKeyPlaySessions][ch].PlaySessions...)
				case EntityKeyGameProgresses:
					out.GameProgresses = append(out.GameProgresses, mergedByID[EntityKeyGameProgresses][ch].GameProgresses...)
				case EntityKeyGameReviews:
					out.GameReviews = append(out.GameReviews, mergedByID[EntityKeyGameReviews][ch].GameReviews...)
				case EntityKeyGameTags:
					out.GameTags = append(out.GameTags, mergedByID[EntityKeyGameTags][ch].GameTags...)
				case EntityKeyGameMetadataSources:
					out.MetadataSources = append(out.MetadataSources, mergedByID[EntityKeyGameMetadataSources][ch].MetadataSources...)
				case EntityKeyGameCategories:
					out.GameCategories = append(out.GameCategories, mergedByID[EntityKeyGameCategories][ch].GameCategories...)
				}
				continue
			}
			// 未变化桶：直接使用本地数据
			bc := localBuckets[entityKey][ch]
			if bc == nil {
				continue
			}
			switch entityKey {
			case EntityKeyGames:
				out.Games = append(out.Games, bc.Games...)
			case EntityKeyPlaySessions:
				out.PlaySessions = append(out.PlaySessions, bc.PlaySessions...)
			case EntityKeyGameProgresses:
				out.GameProgresses = append(out.GameProgresses, bc.GameProgresses...)
			case EntityKeyGameReviews:
				out.GameReviews = append(out.GameReviews, bc.GameReviews...)
			case EntityKeyGameTags:
				out.GameTags = append(out.GameTags, bc.GameTags...)
			case EntityKeyGameMetadataSources:
				out.MetadataSources = append(out.MetadataSources, bc.MetadataSources...)
			case EntityKeyGameCategories:
				out.GameCategories = append(out.GameCategories, bc.GameCategories...)
			}
		}
	}

	sortSnapshot(&out)
	return out
}

// pushBucketKeys 给出最终需要上传的桶 key 列表：本地 hash 与远端 hash 不一致即需要上传。
func pushBucketKeys(final, remote Manifest) []string {
	out := make([]string, 0)
	for _, entityKey := range EntityKeys() {
		for _, ch := range bucketKeysSorted() {
			finalRef := final.Buckets[entityKey][ch]
			remoteRef := remote.Buckets[entityKey][ch]
			if finalRef.Hash != remoteRef.Hash {
				out = append(out, BucketKey(entityKey, ch))
			}
		}
	}
	return out
}

func pushSingletonNames(final, remote Manifest) []string {
	out := make([]string, 0, 2)
	for _, name := range []string{SingletonCategories, SingletonTombstones} {
		if final.Singletons[name].Hash != remote.Singletons[name].Hash {
			out = append(out, name)
		}
	}
	return out
}

// remoteManifestToSnapshot 把 manifest 中的 cover 引用还原成一个壳 Snapshot，供 ReconcileCoverAssets 使用。
// 仅 Covers 字段有意义；其它字段无关。
func remoteManifestToSnapshot(m Manifest) Snapshot {
	covers := make([]CoverAsset, 0, len(m.Covers))
	for _, c := range m.Covers {
		covers = append(covers, CoverAsset{
			GameID:    c.GameID,
			Ext:       c.Ext,
			UpdatedAt: c.UpdatedAt,
		})
	}
	return Snapshot{Covers: covers}
}

func containsString(items []string, target string) bool {
	for _, s := range items {
		if s == target {
			return true
		}
	}
	return false
}

func appendUniqueString(items []string, value string) []string {
	if containsString(items, value) {
		return items
	}
	return append(items, value)
}

func appendBucketIntoSnapshot(s *Snapshot, entityKey string, bc *BucketContent) {
	if bc == nil {
		return
	}
	switch entityKey {
	case EntityKeyGames:
		s.Games = append(s.Games, bc.Games...)
	case EntityKeyPlaySessions:
		s.PlaySessions = append(s.PlaySessions, bc.PlaySessions...)
	case EntityKeyGameProgresses:
		s.GameProgresses = append(s.GameProgresses, bc.GameProgresses...)
	case EntityKeyGameReviews:
		s.GameReviews = append(s.GameReviews, bc.GameReviews...)
	case EntityKeyGameTags:
		s.GameTags = append(s.GameTags, bc.GameTags...)
	case EntityKeyGameMetadataSources:
		s.MetadataSources = append(s.MetadataSources, bc.MetadataSources...)
	case EntityKeyGameCategories:
		s.GameCategories = append(s.GameCategories, bc.GameCategories...)
	}
}

// indexSnapshotByGameBucket 把 mergedSubset 重新按 entity/bucket 分组，方便 assembleFinalSnapshot 提取。
func indexSnapshotByGameBucket(s Snapshot) map[string]map[string]*BucketContent {
	out := make(map[string]map[string]*BucketContent)
	for _, entityKey := range EntityKeys() {
		out[entityKey] = make(map[string]*BucketContent, BucketCount)
		for _, ch := range bucketKeysSorted() {
			out[entityKey][ch] = &BucketContent{}
		}
	}
	for _, g := range s.Games {
		k := BucketKeyOfGame(g.ID)
		out[EntityKeyGames][k].Games = append(out[EntityKeyGames][k].Games, g)
	}
	for _, ps := range s.PlaySessions {
		k := BucketKeyOfGame(ps.GameID)
		out[EntityKeyPlaySessions][k].PlaySessions = append(out[EntityKeyPlaySessions][k].PlaySessions, ps)
	}
	for _, p := range s.GameProgresses {
		k := BucketKeyOfGame(p.GameID)
		out[EntityKeyGameProgresses][k].GameProgresses = append(out[EntityKeyGameProgresses][k].GameProgresses, p)
	}
	for _, review := range s.GameReviews {
		k := BucketKeyOfGame(review.GameID)
		out[EntityKeyGameReviews][k].GameReviews = append(out[EntityKeyGameReviews][k].GameReviews, review)
	}
	for _, t := range s.GameTags {
		k := BucketKeyOfGame(t.GameID)
		out[EntityKeyGameTags][k].GameTags = append(out[EntityKeyGameTags][k].GameTags, t)
	}
	for _, source := range s.MetadataSources {
		k := BucketKeyOfGame(source.GameID)
		out[EntityKeyGameMetadataSources][k].MetadataSources = append(out[EntityKeyGameMetadataSources][k].MetadataSources, source)
	}
	for _, r := range s.GameCategories {
		k := BucketKeyOfGame(r.GameID)
		out[EntityKeyGameCategories][k].GameCategories = append(out[EntityKeyGameCategories][k].GameCategories, r)
	}
	return out
}

// now 是 SyncNow 内部统一使用的时间源，便于测试时替换（当前直接走真实时间）。
func (h *Helper) now() time.Time {
	return time.Now().UTC()
}
