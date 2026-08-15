package cloudsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// BucketKeyOfGame 把 game_id 映射到 0..f 共 16 个桶之一。
// 取小写后的第一个 hex 字符；非 hex 字符（包括空串）兜底归到桶 "0"，保证总能落到一个合法桶。
func BucketKeyOfGame(gameID string) string {
	if gameID == "" {
		return "0"
	}
	for _, r := range strings.ToLower(gameID) {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f':
			return string(r)
		default:
			return "0"
		}
	}
	return "0"
}

// BucketContent 按实体类型聚合一个桶内的本地/远端 items。
// 字段命名与 EntityKey* 常量保持一致，便于在 manifest 中索引。
type BucketContent struct {
	Games           []Game
	PlaySessions    []PlaySession
	GameProgresses  []GameProgress
	GameReviews     []GameReview
	GameTags        []GameTag
	MetadataSources []MetadataSource
	GameCategories  []Relation
}

// EmptyBuckets 返回一组完整的空桶（每种实体 16 个），用于 SyncNow 的初始化。
func EmptyBuckets() map[string]map[string]*BucketContent {
	result := make(map[string]map[string]*BucketContent, len(EntityKeys()))
	for _, entity := range EntityKeys() {
		result[entity] = make(map[string]*BucketContent, BucketCount)
		for i := 0; i < BucketCount; i++ {
			result[entity][string(BucketHexAlphabet[i])] = &BucketContent{}
		}
	}
	return result
}

// Bucketize 把 snapshot 中可分桶的实体按 game_id 首字符路由到 16 个桶。
// categories 与 tombstones 不分桶（量级小、merge 需要全量），不在这里处理。
// 返回值的外层 key 是 entity（如 "games"），内层 key 是 bucket 字符（"0".."f"）。
func Bucketize(snapshot Snapshot) map[string]map[string]*BucketContent {
	buckets := EmptyBuckets()

	for _, g := range snapshot.Games {
		k := BucketKeyOfGame(g.ID)
		buckets[EntityKeyGames][k].Games = append(buckets[EntityKeyGames][k].Games, g)
	}
	for _, s := range snapshot.PlaySessions {
		k := BucketKeyOfGame(s.GameID)
		buckets[EntityKeyPlaySessions][k].PlaySessions = append(buckets[EntityKeyPlaySessions][k].PlaySessions, s)
	}
	for _, p := range snapshot.GameProgresses {
		k := BucketKeyOfGame(p.GameID)
		buckets[EntityKeyGameProgresses][k].GameProgresses = append(buckets[EntityKeyGameProgresses][k].GameProgresses, p)
	}
	for _, review := range snapshot.GameReviews {
		k := BucketKeyOfGame(review.GameID)
		buckets[EntityKeyGameReviews][k].GameReviews = append(buckets[EntityKeyGameReviews][k].GameReviews, review)
	}
	for _, t := range snapshot.GameTags {
		k := BucketKeyOfGame(t.GameID)
		buckets[EntityKeyGameTags][k].GameTags = append(buckets[EntityKeyGameTags][k].GameTags, t)
	}
	for _, source := range snapshot.MetadataSources {
		k := BucketKeyOfGame(source.GameID)
		buckets[EntityKeyGameMetadataSources][k].MetadataSources = append(buckets[EntityKeyGameMetadataSources][k].MetadataSources, source)
	}
	for _, r := range snapshot.GameCategories {
		k := BucketKeyOfGame(r.GameID)
		buckets[EntityKeyGameCategories][k].GameCategories = append(buckets[EntityKeyGameCategories][k].GameCategories, r)
	}

	// 桶内排序，保证 hash 可重复
	for _, byBucket := range buckets {
		for _, bc := range byBucket {
			sortBucket(bc)
		}
	}

	return buckets
}

// Unbucketize 把分桶后的 buckets 拼回完整 Snapshot；
// categories 与 tombstones 从外部传入（它们不分桶）。
func Unbucketize(buckets map[string]map[string]*BucketContent, categories []Category, tombstones []Tombstone) Snapshot {
	out := Snapshot{
		SchemaVersion: SchemaVersion,
		Categories:    append([]Category{}, categories...),
		Tombstones:    append([]Tombstone{}, tombstones...),
	}

	for _, k := range bucketKeysSorted() {
		if bc := buckets[EntityKeyGames][k]; bc != nil {
			out.Games = append(out.Games, bc.Games...)
		}
		if bc := buckets[EntityKeyPlaySessions][k]; bc != nil {
			out.PlaySessions = append(out.PlaySessions, bc.PlaySessions...)
		}
		if bc := buckets[EntityKeyGameProgresses][k]; bc != nil {
			out.GameProgresses = append(out.GameProgresses, bc.GameProgresses...)
		}
		if bc := buckets[EntityKeyGameReviews][k]; bc != nil {
			out.GameReviews = append(out.GameReviews, bc.GameReviews...)
		}
		if bc := buckets[EntityKeyGameTags][k]; bc != nil {
			out.GameTags = append(out.GameTags, bc.GameTags...)
		}
		if bc := buckets[EntityKeyGameMetadataSources][k]; bc != nil {
			out.MetadataSources = append(out.MetadataSources, bc.MetadataSources...)
		}
		if bc := buckets[EntityKeyGameCategories][k]; bc != nil {
			out.GameCategories = append(out.GameCategories, bc.GameCategories...)
		}
	}

	sortSnapshot(&out)
	return out
}

// BucketHash 计算一个桶内某实体类型的内容指纹。
// 流程：sort → canonical JSON → sha256 → 取 hex 前 32 字符。
// time 字段统一截到秒（UTC），消除跨进程的精度抖动。
//
// 调用形式：BucketHash(bc.Games) / BucketHash(bc.PlaySessions) / ...
// 也可用于 Singleton：BucketHash(categories) / BucketHash(tombstones)。
func BucketHash(v any) (string, error) {
	normalized, err := normalizeForHash(v)
	if err != nil {
		return "", err
	}
	buf, err := encodeCanonical(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:32], nil
}

// MarshalBucketFile 把一组 entity items 写成 BucketFile 的 JSON 字节。
// bucketKey 形如 "games/3"。
func MarshalBucketFile(entityKey, bucketChar string, bc *BucketContent) ([]byte, error) {
	file := BucketFile{
		SchemaVersion: SchemaVersion,
		BucketKey:     entityKey + "/" + bucketChar,
	}
	if bc != nil {
		switch entityKey {
		case EntityKeyGames:
			file.Games = bc.Games
		case EntityKeyPlaySessions:
			file.PlaySessions = bc.PlaySessions
		case EntityKeyGameProgresses:
			file.GameProgresses = bc.GameProgresses
		case EntityKeyGameReviews:
			file.GameReviews = bc.GameReviews
		case EntityKeyGameTags:
			file.GameTags = bc.GameTags
		case EntityKeyGameMetadataSources:
			file.MetadataSources = bc.MetadataSources
		case EntityKeyGameCategories:
			file.GameCategories = bc.GameCategories
		default:
			return nil, fmt.Errorf("unknown entity key for bucket marshal: %s", entityKey)
		}
	}
	return json.MarshalIndent(file, "", "  ")
}

// UnmarshalBucketFile 把 BucketFile JSON 字节解析回 (entityKey, bucketChar, BucketContent)。
func UnmarshalBucketFile(raw []byte) (entityKey, bucketChar string, bc BucketContent, err error) {
	var f BucketFile
	if err = json.Unmarshal(raw, &f); err != nil {
		return "", "", BucketContent{}, fmt.Errorf("decode bucket file: %w", err)
	}
	parts := strings.SplitN(f.BucketKey, "/", 2)
	if len(parts) != 2 {
		return "", "", BucketContent{}, fmt.Errorf("invalid bucket_key %q", f.BucketKey)
	}
	entityKey = parts[0]
	bucketChar = parts[1]
	bc.Games = f.Games
	bc.PlaySessions = f.PlaySessions
	bc.GameProgresses = f.GameProgresses
	bc.GameReviews = f.GameReviews
	bc.GameTags = f.GameTags
	bc.MetadataSources = f.MetadataSources
	bc.GameCategories = f.GameCategories
	sortBucket(&bc)
	return entityKey, bucketChar, bc, nil
}

// BucketItemCount 返回某实体类型在桶内的条目数。
func BucketItemCount(entityKey string, bc *BucketContent) int {
	if bc == nil {
		return 0
	}
	switch entityKey {
	case EntityKeyGames:
		return len(bc.Games)
	case EntityKeyPlaySessions:
		return len(bc.PlaySessions)
	case EntityKeyGameProgresses:
		return len(bc.GameProgresses)
	case EntityKeyGameReviews:
		return len(bc.GameReviews)
	case EntityKeyGameTags:
		return len(bc.GameTags)
	case EntityKeyGameMetadataSources:
		return len(bc.MetadataSources)
	case EntityKeyGameCategories:
		return len(bc.GameCategories)
	}
	return 0
}

// BucketHashOf 取出某实体类型在桶内的 items 并计算 hash。
func BucketHashOf(entityKey string, bc *BucketContent) (string, error) {
	if bc == nil {
		return BucketHash([]any{})
	}
	switch entityKey {
	case EntityKeyGames:
		return BucketHash(bc.Games)
	case EntityKeyPlaySessions:
		return BucketHash(bc.PlaySessions)
	case EntityKeyGameProgresses:
		return BucketHash(bc.GameProgresses)
	case EntityKeyGameReviews:
		return BucketHash(bc.GameReviews)
	case EntityKeyGameTags:
		return BucketHash(bc.GameTags)
	case EntityKeyGameMetadataSources:
		return BucketHash(bc.MetadataSources)
	case EntityKeyGameCategories:
		return BucketHash(bc.GameCategories)
	}
	return "", fmt.Errorf("unknown entity key: %s", entityKey)
}

// BucketKey 把 (entity, char) 组合成 manifest 引用 key（"games/3"）。
func BucketKey(entityKey, bucketChar string) string {
	return entityKey + "/" + bucketChar
}

// bucketKeysSorted 返回 "0".."f" 的固定顺序，避免 map 遍历的随机性影响序列化。
func bucketKeysSorted() []string {
	out := make([]string, 0, BucketCount)
	for i := 0; i < BucketCount; i++ {
		out = append(out, string(BucketHexAlphabet[i]))
	}
	return out
}

// sortBucket 对桶内每种实体按主键排序。
// 与 sortSnapshot 的排序保持一致，保证 Bucketize 后做 hash 是确定性的。
func sortBucket(bc *BucketContent) {
	if bc == nil {
		return
	}
	sort.Slice(bc.Games, func(i, j int) bool { return bc.Games[i].ID < bc.Games[j].ID })
	sort.Slice(bc.PlaySessions, func(i, j int) bool { return bc.PlaySessions[i].ID < bc.PlaySessions[j].ID })
	sort.Slice(bc.GameProgresses, func(i, j int) bool { return bc.GameProgresses[i].ID < bc.GameProgresses[j].ID })
	sort.Slice(bc.GameReviews, func(i, j int) bool { return bc.GameReviews[i].GameID < bc.GameReviews[j].GameID })
	sort.Slice(bc.GameTags, func(i, j int) bool {
		return tagTombstoneID(bc.GameTags[i].GameID, bc.GameTags[i].Source, bc.GameTags[i].Name) <
			tagTombstoneID(bc.GameTags[j].GameID, bc.GameTags[j].Source, bc.GameTags[j].Name)
	})
	sort.Slice(bc.MetadataSources, func(i, j int) bool {
		return metadataSourceTombstoneID(bc.MetadataSources[i].GameID, bc.MetadataSources[i].SourceType) <
			metadataSourceTombstoneID(bc.MetadataSources[j].GameID, bc.MetadataSources[j].SourceType)
	})
	sort.Slice(bc.GameCategories, func(i, j int) bool {
		left := bc.GameCategories[i].GameID + "::" + bc.GameCategories[i].CategoryID
		right := bc.GameCategories[j].GameID + "::" + bc.GameCategories[j].CategoryID
		return left < right
	})
}

// normalizeForHash 把输入归一化为可重复 hash 的中间形态：
//   - 任何 time.Time 截到秒并转为 UTC RFC3339；
//   - nil slice 与 []T{} 都视作空数组（让"无数据"在 hash 上等价）；
//   - slice 已排序的前提下，每个元素递归处理。
//
// 实现上走 json round-trip 拿到普通 map/slice，再统一时间字段。
func normalizeForHash(v any) (any, error) {
	if v == nil {
		return []any{}, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("pre-hash marshal: %w", err)
	}
	// nil slice 序列化为 "null"，统一视作空数组，避免 nil vs empty 产生不同 hash
	if string(raw) == "null" {
		return []any{}, nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("pre-hash unmarshal: %w", err)
	}
	return normalizeTimes(generic), nil
}

// normalizeTimes 递归遍历 generic JSON 树，把任何看起来是 RFC3339 时间字符串的值统一截到秒 UTC。
// 这一步是为了消除 DuckDB 不同读取路径返回 time 时纳秒/微秒精度差异带来的 hash 抖动。
func normalizeTimes(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			t[k] = normalizeTimes(vv)
		}
		return t
	case []any:
		for i, vv := range t {
			t[i] = normalizeTimes(vv)
		}
		return t
	case string:
		if ts, ok := tryParseTime(t); ok {
			return ts.UTC().Truncate(time.Second).Format(time.RFC3339)
		}
		return t
	default:
		return v
	}
}

func tryParseTime(s string) (time.Time, bool) {
	if len(s) < 19 {
		return time.Time{}, false
	}
	// 仅尝试 RFC3339 系列格式，避免误判普通字符串
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// encodeCanonical 输出确定性 JSON：禁用 HTML 转义，对象 key 按字典序输出。
// 由于 encoding/json 默认就按 map key 排序输出，这里只需关闭 HTML 转义并去掉末尾换行。
func encodeCanonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("canonical encode: %w", err)
	}
	out := buf.Bytes()
	// json.Encoder 会在末尾追加 '\n'，去掉以保持稳定
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return out, nil
}
