package service

import (
	"context"
	"reflect"
	"testing"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service/gamehelper"
	"lunabox/backend/utils/metadata"
)

func TestApplyRemoteMetadataMergesAliases(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewGameService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	existing := models.Game{
		ID:      "alias-refresh-game",
		Name:    "本地名称",
		Aliases: []string{"手动简称", "SubaHibi"},
	}
	if err := service.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Game: existing}); err != nil {
		t.Fatalf("add game: %v", err)
	}
	existing, err := service.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}

	fields := gamehelper.NormalizeMetadataUpdateFields([]enums.MetadataUpdateField{
		enums.MetadataUpdateFieldAliases,
	})
	_, err = service.applyRemoteMetadataResult(existing, metadata.MetadataResult{
		Game: models.Game{
			Name:    "远端名称",
			Aliases: []string{"subahibi", "素晴らしき日々～不連続存在～"},
		},
	}, false, fields)
	if err != nil {
		t.Fatalf("apply remote metadata: %v", err)
	}

	saved, err := service.GetGameByID(existing.ID)
	if err != nil {
		t.Fatalf("get updated game: %v", err)
	}
	wantAliases := []string{"手动简称", "SubaHibi", "素晴らしき日々～不連続存在～"}
	if !reflect.DeepEqual(saved.Aliases, wantAliases) {
		t.Fatalf("aliases: got %#v want %#v", saved.Aliases, wantAliases)
	}
	if saved.Name != existing.Name {
		t.Fatalf("unselected name changed: got %q want %q", saved.Name, existing.Name)
	}
}

func TestUpdateDownloadedCoverURLSkipsSupersededSource(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewGameService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	if _, err := db.Exec(`
		INSERT INTO games (
			id, name, cover_url, cover_source_url, status, source_type,
			cached_at, created_at, updated_at
		) VALUES ('cover-race', 'Cover Race', '/local/covers/current.webp',
			'https://example.com/current.webp', 'not_started', 'local',
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`); err != nil {
		t.Fatalf("insert game: %v", err)
	}

	updated, err := service.updateDownloadedCoverURL(
		context.Background(),
		"cover-race",
		"/local/covers/stale.webp",
		"https://example.com/stale.webp",
	)
	if err != nil {
		t.Fatalf("update stale cover: %v", err)
	}
	if updated {
		t.Fatal("stale cover update unexpectedly changed the game")
	}

	updated, err = service.updateDownloadedCoverURL(
		context.Background(),
		"cover-race",
		"/local/covers/current-new.webp",
		"https://example.com/current.webp",
	)
	if err != nil {
		t.Fatalf("update current cover: %v", err)
	}
	if !updated {
		t.Fatal("current cover update was skipped")
	}

	var coverURL string
	if err := db.QueryRow(`SELECT cover_url FROM games WHERE id = 'cover-race'`).Scan(&coverURL); err != nil {
		t.Fatalf("query cover URL: %v", err)
	}
	if coverURL != "/local/covers/current-new.webp" {
		t.Fatalf("cover URL = %q", coverURL)
	}

	if err := service.updateCoverURL("cover-race", "/local/covers/manual.webp"); err != nil {
		t.Fatalf("update manual cover: %v", err)
	}
	var coverSourceURL string
	if err := db.QueryRow(`
		SELECT cover_url, COALESCE(cover_source_url, '')
		FROM games WHERE id = 'cover-race'
	`).Scan(&coverURL, &coverSourceURL); err != nil {
		t.Fatalf("query manual cover: %v", err)
	}
	if coverURL != "/local/covers/manual.webp" || coverSourceURL != "" {
		t.Fatalf("manual cover = %q source = %q", coverURL, coverSourceURL)
	}
}

// 手动添加时填写的用户标签随游戏一起保存，去掉重复的。
func TestAddGameFromWebMetadataSavesUserTags(t *testing.T) {
	db := setupImportServiceTestDB(t)
	service := NewGameService()
	service.Init(context.Background(), db, &appconf.AppConfig{})
	tags := NewTagService()
	tags.Init(context.Background(), db, &appconf.AppConfig{})
	service.SetTagService(tags)

	game := models.Game{ID: "manual-tags-game", Name: "手动添加", SourceType: enums.Local}
	err := service.AddGameFromWebMetadata(vo.GameMetadataFromWebVO{Source: enums.Local, Game: game, Tags: []metadata.TagItem{
		{Name: "纯爱", Source: "user"}, {Name: " 纯爱 ", Source: "user"}, {Name: "乐队", Source: "user"},
	}})
	if err != nil {
		t.Fatalf("add game: %v", err)
	}
	saved, err := tags.GetTagsByGame(game.ID)
	if err != nil {
		t.Fatalf("get tags: %v", err)
	}
	var names []string
	for _, tag := range saved {
		if tag.Source != "user" {
			t.Fatalf("tag %q has source %q", tag.Name, tag.Source)
		}
		names = append(names, tag.Name)
	}
	if len(names) != 2 {
		t.Fatalf("user tags: %q", names)
	}
}
