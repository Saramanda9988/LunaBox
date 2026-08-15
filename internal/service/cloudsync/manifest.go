package cloudsync

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// BuildManifestFromBuckets 基于分桶后的本地数据装配出 manifest（本地侧视图）。
// revisionID 由调用方决定：迁移/上传新版本时需要新生成；用于"读出本地视角"时可以传空。
func BuildManifestFromBuckets(
	buckets map[string]map[string]*BucketContent,
	categories []Category,
	tombstones []Tombstone,
	covers []CoverAsset,
	deviceID string,
	revisionID string,
	exportedAt time.Time,
) (Manifest, error) {
	if revisionID == "" {
		revisionID = uuid.New().String()
	}
	m := Manifest{
		SchemaVersion: SchemaVersion,
		RevisionID:    revisionID,
		ExportedAt:    exportedAt.UTC(),
		DeviceID:      deviceID,
		Buckets:       make(map[string]map[string]BucketRef, len(EntityKeys())),
		Singletons:    make(map[string]BucketRef, 2),
		Covers:        make([]CoverRef, 0, len(covers)),
	}

	for _, entityKey := range EntityKeys() {
		byBucket := buckets[entityKey]
		m.Buckets[entityKey] = make(map[string]BucketRef, BucketCount)
		for _, ch := range bucketKeysSorted() {
			bc := byBucket[ch]
			hash, err := BucketHashOf(entityKey, bc)
			if err != nil {
				return Manifest{}, fmt.Errorf("hash bucket %s/%s: %w", entityKey, ch, err)
			}
			m.Buckets[entityKey][ch] = BucketRef{
				Hash:      hash,
				Count:     BucketItemCount(entityKey, bc),
				UpdatedAt: latestUpdatedAtInBucket(entityKey, bc),
			}
		}
	}

	// Singletons
	catHash, err := BucketHash(categories)
	if err != nil {
		return Manifest{}, fmt.Errorf("hash categories: %w", err)
	}
	m.Singletons[SingletonCategories] = BucketRef{
		Hash:      catHash,
		Count:     len(categories),
		UpdatedAt: latestUpdatedAtCategories(categories),
	}
	tsHash, err := BucketHash(tombstones)
	if err != nil {
		return Manifest{}, fmt.Errorf("hash tombstones: %w", err)
	}
	m.Singletons[SingletonTombstones] = BucketRef{
		Hash:      tsHash,
		Count:     len(tombstones),
		UpdatedAt: latestUpdatedAtTombstones(tombstones),
	}

	// Covers：稳定排序 + 内容指纹仅依赖 game_id + ext + updated_at（秒精度）
	sortedCovers := append([]CoverAsset{}, covers...)
	sort.Slice(sortedCovers, func(i, j int) bool { return sortedCovers[i].GameID < sortedCovers[j].GameID })
	for _, c := range sortedCovers {
		hash, err := BucketHash([]CoverAsset{c})
		if err != nil {
			return Manifest{}, fmt.Errorf("hash cover %s: %w", c.GameID, err)
		}
		m.Covers = append(m.Covers, CoverRef{
			GameID:    c.GameID,
			Ext:       c.Ext,
			UpdatedAt: c.UpdatedAt.UTC().Truncate(time.Second),
			Hash:      hash,
		})
	}

	return m, nil
}

// BucketDiff 描述一次同步前的差异计划。
type BucketDiff struct {
	// ToPull 是远端 hash 与本地缓存 remote_hash 不一致的桶 key 列表（形如 "games/3"）；这些桶需要从远端下载后参与 merge。
	ToPull []string
	// LocalChanged 是本地新算 hash 与缓存 local_hash 不一致的桶 key 列表；这些桶需要重新上传（合并完成后再决定）。
	LocalChanged []string
	// SingletonsToPull 是远端 hash 与本地缓存不一致的 singleton 名（"categories" / "tombstones"）。
	SingletonsToPull []string
	// SingletonsChanged 是本地 hash 与本地缓存不一致的 singleton 名。
	SingletonsChanged []string
	// CoversChanged 是本地与远端 manifest 中封面引用不一致的游戏 ID。
	// 封面没有独立的 sync_state 行，因此直接比较两侧 manifest。
	CoversChanged []string
}

// HasWork 返回是否有任何拉取或本地变化，决定是否进入 merge 流程。
func (d BucketDiff) HasWork() bool {
	return len(d.ToPull)+len(d.LocalChanged)+len(d.SingletonsToPull)+len(d.SingletonsChanged)+len(d.CoversChanged) > 0
}

// DiffBuckets 比较"本地新算 manifest"、"本地缓存 cloud_sync_state"、"远端 manifest"，给出本次同步要做的拉/推清单。
//
// 判定规则：
//   - 远端 manifest 的桶 hash 与缓存 remote_hash 不一致 → 需要拉取（远端有新变化）。
//   - 本地新算 hash 与缓存 local_hash 不一致 → 本地有改动，需要参与合并并最终上传。
//   - 两边都没动 → 跳过。
//
// remoteManifest 可以是零值（表示远端无 manifest），此时所有本地非空桶被视为 LocalChanged。
func DiffBuckets(local Manifest, cached map[string]SyncStateRow, remote Manifest, remoteExists bool) BucketDiff {
	out := BucketDiff{}

	for _, entityKey := range EntityKeys() {
		for _, ch := range bucketKeysSorted() {
			key := BucketKey(entityKey, ch)
			localRef := local.Buckets[entityKey][ch]
			cachedRow, hasCache := cached[key]

			// 远端侧
			if remoteExists {
				remoteRef := remote.Buckets[entityKey][ch]
				if !hasCache && remoteRef.Count > 0 {
					out.ToPull = append(out.ToPull, key)
				} else if hasCache && cachedRow.RemoteHash != remoteRef.Hash {
					out.ToPull = append(out.ToPull, key)
				}
			}

			// 本地侧
			if !hasCache {
				if localRef.Count > 0 {
					out.LocalChanged = append(out.LocalChanged, key)
				}
			} else if cachedRow.LocalHash != localRef.Hash {
				out.LocalChanged = append(out.LocalChanged, key)
			}
		}
	}

	// Singletons
	for _, name := range []string{SingletonCategories, SingletonTombstones} {
		stateKey := SingletonStateKey(name)
		localRef := local.Singletons[name]
		cachedRow, hasCache := cached[stateKey]

		if remoteExists {
			remoteRef := remote.Singletons[name]
			if !hasCache && remoteRef.Count > 0 {
				out.SingletonsToPull = append(out.SingletonsToPull, name)
			} else if hasCache && cachedRow.RemoteHash != remoteRef.Hash {
				out.SingletonsToPull = append(out.SingletonsToPull, name)
			}
		}

		if !hasCache {
			if localRef.Count > 0 {
				out.SingletonsChanged = append(out.SingletonsChanged, name)
			}
		} else if cachedRow.LocalHash != localRef.Hash {
			out.SingletonsChanged = append(out.SingletonsChanged, name)
		}
	}

	if remoteExists {
		localCovers := make(map[string]CoverRef, len(local.Covers))
		remoteCovers := make(map[string]CoverRef, len(remote.Covers))
		for _, cover := range local.Covers {
			localCovers[cover.GameID] = cover
		}
		for _, cover := range remote.Covers {
			remoteCovers[cover.GameID] = cover
		}
		coverIDs := make(map[string]struct{}, len(localCovers)+len(remoteCovers))
		for gameID := range localCovers {
			coverIDs[gameID] = struct{}{}
		}
		for gameID := range remoteCovers {
			coverIDs[gameID] = struct{}{}
		}
		for gameID := range coverIDs {
			localCover, hasLocal := localCovers[gameID]
			remoteCover, hasRemote := remoteCovers[gameID]
			if hasLocal != hasRemote || localCover.Hash != remoteCover.Hash || localCover.Ext != remoteCover.Ext || !localCover.UpdatedAt.Equal(remoteCover.UpdatedAt) {
				out.CoversChanged = append(out.CoversChanged, gameID)
			}
		}
	}

	sort.Strings(out.ToPull)
	sort.Strings(out.LocalChanged)
	sort.Strings(out.SingletonsToPull)
	sort.Strings(out.SingletonsChanged)
	sort.Strings(out.CoversChanged)
	return out
}

// EncodeManifest 输出确定性 JSON 字节（缩进 2 空格，便于人工排查；map key 在 Go 中按字典序输出）。
func EncodeManifest(m Manifest) ([]byte, error) {
	buf, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return buf, nil
}

// DecodeManifest 反序列化远端 manifest 字节。
func DecodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if m.Buckets == nil {
		m.Buckets = map[string]map[string]BucketRef{}
	}
	if m.Singletons == nil {
		m.Singletons = map[string]BucketRef{}
	}
	return m, nil
}

// latestUpdatedAtInBucket 取桶内某实体类型的最大 updated_at；用于 manifest 元数据。
// hash 已能识别变更，updated_at 仅为可读性服务。
func latestUpdatedAtInBucket(entityKey string, bc *BucketContent) time.Time {
	if bc == nil {
		return time.Time{}
	}
	var latest time.Time
	switch entityKey {
	case EntityKeyGames:
		for _, g := range bc.Games {
			if g.UpdatedAt.After(latest) {
				latest = g.UpdatedAt
			}
		}
	case EntityKeyPlaySessions:
		for _, s := range bc.PlaySessions {
			if s.UpdatedAt.After(latest) {
				latest = s.UpdatedAt
			}
		}
	case EntityKeyGameProgresses:
		for _, p := range bc.GameProgresses {
			if p.UpdatedAt.After(latest) {
				latest = p.UpdatedAt
			}
		}
	case EntityKeyGameReviews:
		for _, review := range bc.GameReviews {
			if review.UpdatedAt.After(latest) {
				latest = review.UpdatedAt
			}
		}
	case EntityKeyGameTags:
		for _, t := range bc.GameTags {
			if t.UpdatedAt.After(latest) {
				latest = t.UpdatedAt
			}
		}
	case EntityKeyGameMetadataSources:
		for _, source := range bc.MetadataSources {
			if source.UpdatedAt.After(latest) {
				latest = source.UpdatedAt
			}
		}
	case EntityKeyGameCategories:
		for _, r := range bc.GameCategories {
			if r.UpdatedAt.After(latest) {
				latest = r.UpdatedAt
			}
		}
	}
	return latest.UTC().Truncate(time.Second)
}

func latestUpdatedAtCategories(items []Category) time.Time {
	var latest time.Time
	for _, c := range items {
		if c.UpdatedAt.After(latest) {
			latest = c.UpdatedAt
		}
	}
	return latest.UTC().Truncate(time.Second)
}

func latestUpdatedAtTombstones(items []Tombstone) time.Time {
	var latest time.Time
	for _, t := range items {
		if t.DeletedAt.After(latest) {
			latest = t.DeletedAt
		}
	}
	return latest.UTC().Truncate(time.Second)
}
