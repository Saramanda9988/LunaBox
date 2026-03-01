package vnite

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const (
	docStorePrefix = "ÿdocument-storeÿ"
	bySeqPrefix    = "ÿby-sequenceÿ"
	attachBinary   = "ÿattach-binary-storeÿ"
)

type docMeta struct {
	ID      string         `json:"id"`
	Deleted bool           `json:"deleted"`
	Seq     int            `json:"seq"`
	Rev     string         `json:"rev"`
	RevMap  map[string]int `json:"rev_map"`
}

// ExportData 聚合了 vnite 导出的 game / game-local 文档。
type ExportData struct {
	GameDocs      map[string]GameDoc
	GameLocalDocs map[string]GameLocalDoc
}

type GameDoc struct {
	ID          string                    `json:"_id"`
	Metadata    GameMetadata              `json:"metadata"`
	Record      GameRecord                `json:"record"`
	Save        json.RawMessage           `json:"save"`
	Memory      json.RawMessage           `json:"memory"`
	Attachments map[string]AttachmentMeta `json:"_attachments"`
}

type AttachmentMeta struct {
	ContentType string `json:"content_type"`
	Digest      string `json:"digest"`
	Length      int    `json:"length"`
	Stub        bool   `json:"stub"`
}

type GameMetadata struct {
	Name         string   `json:"name"`
	OriginalName string   `json:"originalName"`
	Description  string   `json:"description"`
	Developers   []string `json:"developers"`
	Publishers   []string `json:"publishers"`
	SteamID      string   `json:"steamId"`
	VNDBID       string   `json:"vndbId"`
	YmgalID      string   `json:"ymgalId"`
	BangumiID    string   `json:"bangumiId"`
}

type GameRecord struct {
	AddDate     string      `json:"addDate"`
	LastRunDate string      `json:"lastRunDate"`
	PlayTime    float64     `json:"playTime"`
	PlayStatus  string      `json:"playStatus"`
	Timers      []GameTimer `json:"timers"`
}

type GameTimer struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type GameLocalDoc struct {
	ID       string          `json:"_id"`
	Path     GameLocalPath   `json:"path"`
	Launcher GameLauncher    `json:"launcher"`
	Utils    json.RawMessage `json:"utils"`
}

type GameLocalPath struct {
	GamePath  string   `json:"gamePath"`
	SavePaths []string `json:"savePaths"`
}

type GameLauncher struct {
	Mode         string          `json:"mode"`
	FileConfig   GameFileConfig  `json:"fileConfig"`
	URLConfig    json.RawMessage `json:"urlConfig"`
	ScriptConfig json.RawMessage `json:"scriptConfig"`
	UseMagpie    bool            `json:"useMagpie"`
}

type GameFileConfig struct {
	Path string `json:"path"`
}

func LoadExportData(rootDir string) (*ExportData, error) {
	gamePath := filepath.Join(rootDir, "game")
	localPath := filepath.Join(rootDir, "game-local")

	gameDocs, err := loadTypedDocs[GameDoc](gamePath)
	if err != nil {
		return nil, fmt.Errorf("读取 game 数据失败: %w", err)
	}

	gameLocalDocs, err := loadTypedDocs[GameLocalDoc](localPath)
	if err != nil {
		return nil, fmt.Errorf("读取 game-local 数据失败: %w", err)
	}

	return &ExportData{
		GameDocs:      gameDocs,
		GameLocalDocs: gameLocalDocs,
	}, nil
}

func loadTypedDocs[T any](dbPath string) (map[string]T, error) {
	rawDocs, err := loadRawDocs(dbPath)
	if err != nil {
		return nil, err
	}

	result := make(map[string]T, len(rawDocs))
	for id, raw := range rawDocs {
		var doc T
		if err := json.Unmarshal(raw, &doc); err != nil {
			continue
		}
		result[id] = doc
	}

	return result, nil
}

func loadRawDocs(dbPath string) (map[string]json.RawMessage, error) {
	db, err := leveldb.OpenFile(dbPath, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return nil, err
	}
	defer db.Close()

	docs := map[string]json.RawMessage{}
	iter := db.NewIterator(util.BytesPrefix([]byte(docStorePrefix)), nil)
	defer iter.Release()

	for iter.Next() {
		var meta docMeta
		if err := json.Unmarshal(iter.Value(), &meta); err != nil {
			continue
		}

		if meta.Deleted || meta.ID == "" {
			continue
		}

		seq := meta.Seq
		if seq == 0 && meta.Rev != "" && meta.RevMap != nil {
			seq = meta.RevMap[meta.Rev]
		}
		if seq <= 0 {
			continue
		}

		seqKey := []byte(fmt.Sprintf("%s%016d", bySeqPrefix, seq))
		value, err := db.Get(seqKey, nil)
		if err != nil {
			continue
		}

		if !json.Valid(value) {
			continue
		}

		raw := make([]byte, len(value))
		copy(raw, value)
		docs[meta.ID] = raw
	}

	if err := iter.Error(); err != nil {
		return nil, err
	}

	return docs, nil
}

func LoadGameCoverBytes(rootDir string, gameDoc GameDoc) ([]byte, string, error) {
	attachmentName, attachmentMeta, ok := pickCoverAttachment(gameDoc)
	if !ok || attachmentMeta.Digest == "" {
		return nil, "", nil
	}

	dbPath := filepath.Join(rootDir, "game")
	db, err := leveldb.OpenFile(dbPath, &opt.Options{ReadOnly: true, ErrorIfMissing: true})
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	key := []byte(attachBinary + attachmentMeta.Digest)
	value, err := db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	ext := filepath.Ext(attachmentName)
	if ext == "" {
		ext = inferExtByContentType(attachmentMeta.ContentType)
	}

	raw := make([]byte, len(value))
	copy(raw, value)
	return raw, ext, nil
}

func pickCoverAttachment(gameDoc GameDoc) (string, AttachmentMeta, bool) {
	if len(gameDoc.Attachments) == 0 {
		return "", AttachmentMeta{}, false
	}

	priority := []string{"images_cover.webp", "cover.webp", "cover.jpg", "cover.jpeg", "cover.png"}
	for _, name := range priority {
		if meta, ok := gameDoc.Attachments[name]; ok {
			return name, meta, true
		}
	}

	for name, meta := range gameDoc.Attachments {
		if strings.Contains(strings.ToLower(name), "cover") {
			return name, meta, true
		}
	}

	for name, meta := range gameDoc.Attachments {
		if strings.HasPrefix(strings.ToLower(meta.ContentType), "image/") {
			return name, meta, true
		}
	}

	for name, meta := range gameDoc.Attachments {
		return name, meta, true
	}

	return "", AttachmentMeta{}, false
}

func inferExtByContentType(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/webp":
		return ".webp"
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}
