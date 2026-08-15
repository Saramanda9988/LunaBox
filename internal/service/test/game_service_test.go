package test

import (
	"context"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

// createTestGame 创建测试游戏数据
func createTestGame() models.Game {
	return models.Game{
		ID:         "test-game-001",
		Name:       "测试游戏",
		CoverURL:   "",
		Company:    "测试公司",
		Summary:    "这是一个测试游戏",
		Path:       "C:\\Games\\TestGame\\game.exe",
		SourceType: enums.Local,
		SourceID:   "local-001",
		CreatedAt:  time.Now(),
		CachedAt:   time.Now(),
	}
}

func TestGameService_AddGame(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功添加游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "add-test-001"

		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 验证游戏已添加
		savedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Fatalf("获取游戏失败: %v", err)
		}

		if savedGame.Name != game.Name {
			t.Errorf("游戏名称不匹配: 期望 %s, 得到 %s", game.Name, savedGame.Name)
		}
		if savedGame.Company != game.Company {
			t.Errorf("公司名称不匹配: 期望 %s, 得到 %s", game.Company, savedGame.Company)
		}
	})

	t.Run("自动生成ID", func(t *testing.T) {
		game := createTestGame()
		game.ID = "" // 不提供ID

		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 验证至少有一个游戏被添加（由于ID是自动生成的，无法直接验证）
		resp, err := gameService.GetGames(vo.GameListRequest{})
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(resp.Games) < 1 {
			t.Error("未找到添加的游戏")
		}
	})
}

func TestGameService_GetGameByID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功获取游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "get-test-001"

		// 先添加游戏
		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 获取游戏
		savedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Fatalf("获取游戏失败: %v", err)
		}

		if savedGame.ID != game.ID {
			t.Errorf("游戏ID不匹配: 期望 %s, 得到 %s", game.ID, savedGame.ID)
		}
		if savedGame.Name != game.Name {
			t.Errorf("游戏名称不匹配: 期望 %s, 得到 %s", game.Name, savedGame.Name)
		}
	})

	t.Run("游戏不存在", func(t *testing.T) {
		_, err := gameService.GetGameByID("non-existent-id")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
	})
}

func TestGameService_ManagesMultipleMetadataSources(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})
	game := createTestGame()
	game.ID = "multi-source-game"
	if err := addGameViaMetadata(gameService, game); err != nil {
		t.Fatalf("add game: %v", err)
	}

	if err := gameService.UpsertGameMetadataSource(game.ID, enums.Bangumi, "101"); err != nil {
		t.Fatalf("add Bangumi source: %v", err)
	}
	if err := gameService.UpsertGameMetadataSource(game.ID, enums.Hikarinagi, "hikari-202"); err != nil {
		t.Fatalf("add Hikarinagi source: %v", err)
	}
	if err := gameService.SetDefaultMetadataSource(game.ID, enums.Hikarinagi); err != nil {
		t.Fatalf("set default source: %v", err)
	}

	saved, err := gameService.GetGameByID(game.ID)
	if err != nil {
		t.Fatalf("get game: %v", err)
	}
	if len(saved.MetadataSources) != 2 {
		t.Fatalf("expected two metadata sources, got %d", len(saved.MetadataSources))
	}
	if saved.SourceType != enums.Hikarinagi || saved.SourceID != "hikari-202" {
		t.Fatalf("unexpected default source: %s/%s", saved.SourceType, saved.SourceID)
	}
	if err := gameService.UpsertGameMetadataSource(game.ID, enums.Hikarinagi, "hikari-303"); err != nil {
		t.Fatalf("update default source ID: %v", err)
	}
	saved, err = gameService.GetGameByID(game.ID)
	if err != nil {
		t.Fatalf("get game after default source ID update: %v", err)
	}
	if saved.SourceID != "hikari-303" {
		t.Fatalf("default source ID projection was not updated: %s", saved.SourceID)
	}

	if err := gameService.DeleteGameMetadataSource(game.ID, enums.Hikarinagi); err != nil {
		t.Fatalf("delete default source: %v", err)
	}
	saved, err = gameService.GetGameByID(game.ID)
	if err != nil {
		t.Fatalf("get game after deletion: %v", err)
	}
	if len(saved.MetadataSources) != 1 || saved.SourceType != enums.Bangumi || saved.SourceID != "101" {
		t.Fatalf("unexpected fallback source after deletion: %#v", saved)
	}

	var tombstoneCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sync_tombstones WHERE entity_type = 'game_metadata_source' AND entity_id = ?`, game.ID+"::hikarinagi").Scan(&tombstoneCount); err != nil {
		t.Fatalf("query metadata source tombstone: %v", err)
	}
	if tombstoneCount != 1 {
		t.Fatalf("expected one metadata source tombstone, got %d", tombstoneCount)
	}
}

func TestGameService_RejectsDuplicateInitialMetadataSources(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})
	game := createTestGame()
	game.ID = "duplicate-initial-source-game"
	game.SourceType = enums.Bangumi
	game.SourceID = "101"
	game.MetadataSources = []models.GameMetadataSource{
		{SourceType: enums.Bangumi, SourceID: "101"},
		{SourceType: enums.Bangumi, SourceID: "202"},
	}

	err := addGameViaMetadata(gameService, game)
	if err == nil {
		t.Fatal("expected duplicate metadata sources to be rejected")
	}
	if !strings.Contains(err.Error(), "bangumi 元数据记录存在多个") {
		t.Fatalf("unexpected duplicate source error: %v", err)
	}

	var count int
	if queryErr := db.QueryRow(`SELECT COUNT(*) FROM games WHERE id = ?`, game.ID).Scan(&count); queryErr != nil {
		t.Fatalf("query rejected game: %v", queryErr)
	}
	if count != 0 {
		t.Fatalf("rejected game was persisted: %d", count)
	}
}

func TestGameService_PersistsInitialMetadataSourcesWithSelectedDefault(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})
	game := createTestGame()
	game.ID = "selected-default-source-game"
	game.Name = "来自 Hikarinagi 的主体元数据"
	game.SourceType = enums.Hikarinagi
	game.SourceID = "202"
	game.MetadataSources = []models.GameMetadataSource{
		{SourceType: enums.Bangumi, SourceID: "101"},
		{SourceType: enums.Hikarinagi, SourceID: "202"},
	}

	if err := addGameViaMetadata(gameService, game); err != nil {
		t.Fatalf("add game with initial metadata sources: %v", err)
	}

	saved, err := gameService.GetGameByID(game.ID)
	if err != nil {
		t.Fatalf("get saved game: %v", err)
	}
	if saved.Name != game.Name {
		t.Fatalf("unexpected saved metadata: %q", saved.Name)
	}
	if saved.SourceType != enums.Hikarinagi || saved.SourceID != "202" {
		t.Fatalf("unexpected selected default source: %s/%s", saved.SourceType, saved.SourceID)
	}
	if len(saved.MetadataSources) != 2 {
		t.Fatalf("expected all metadata sources to be saved, got %+v", saved.MetadataSources)
	}
}

func TestGameService_AddGameFromWebMetadataPersistsLaunchFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	game := models.Game{
		ID:                "launch-fields-game",
		Name:              "Launch Fields Game",
		Path:              `D:\Games\launch\game.exe`,
		SavePath:          `D:\Saves\launch`,
		ProcessName:       "actual.exe",
		ReleaseDate:       "2025-02-03",
		UseLocaleEmulator: true,
		UseMagpie:         true,
		SourceType:        enums.Local,
		Status:            enums.StatusWantToPlay,
	}

	if err := addGameViaMetadata(gameService, game); err != nil {
		t.Fatalf("AddGameFromWebMetadata failed: %v", err)
	}

	saved, err := gameService.GetGameByID(game.ID)
	if err != nil {
		t.Fatalf("GetGameByID failed: %v", err)
	}

	if saved.Path != game.Path {
		t.Fatalf("expected path %q, got %q", game.Path, saved.Path)
	}
	if saved.SavePath != game.SavePath {
		t.Fatalf("expected save path %q, got %q", game.SavePath, saved.SavePath)
	}
	if saved.ProcessName != game.ProcessName {
		t.Fatalf("expected process name %q, got %q", game.ProcessName, saved.ProcessName)
	}
	if saved.Status != game.Status {
		t.Fatalf("expected status %q, got %q", game.Status, saved.Status)
	}
	if saved.ReleaseDate != game.ReleaseDate {
		t.Fatalf("expected release date %q, got %q", game.ReleaseDate, saved.ReleaseDate)
	}
	if !saved.UseLocaleEmulator {
		t.Fatal("expected Locale Emulator flag to be persisted")
	}
	if !saved.UseMagpie {
		t.Fatal("expected Magpie flag to be persisted")
	}
}

func TestGameService_GetGames(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("获取所有游戏", func(t *testing.T) {
		// 添加多个游戏
		for i := 1; i <= 3; i++ {
			game := createTestGame()
			game.ID = string(rune('0' + i))
			game.Name = game.Name + string(rune('0'+i))
			err := addGameViaMetadata(gameService, game)
			if err != nil {
				t.Fatalf("添加游戏 %d 失败: %v", i, err)
			}
		}

		resp, err := gameService.GetGames(vo.GameListRequest{})
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(resp.Games) != 3 {
			t.Errorf("期望获取 3 个游戏, 实际获取 %d 个", len(resp.Games))
		}
		if resp.Total != 3 {
			t.Errorf("期望 total 为 3, 实际为 %d", resp.Total)
		}
	})

	t.Run("空列表", func(t *testing.T) {
		// 使用新的数据库
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		resp, err := newService.GetGames(vo.GameListRequest{})
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(resp.Games) != 0 {
			t.Errorf("期望空列表, 实际获取 %d 个游戏", len(resp.Games))
		}
	})

	t.Run("返回最近游玩时间", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		game := createTestGame()
		game.ID = "last-played-test-001"
		if err := addGameViaMetadata(newService, game); err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		olderStart := time.Now().Add(-4 * time.Hour)
		newerStart := time.Now().Add(-90 * time.Minute)
		if _, err := newDB.Exec(
			"INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
			"ps-last-played-older",
			game.ID,
			olderStart,
			olderStart.Add(30*time.Minute),
			1800,
		); err != nil {
			t.Fatalf("插入旧游玩记录失败: %v", err)
		}
		if _, err := newDB.Exec(
			"INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
			"ps-last-played-newer",
			game.ID,
			newerStart,
			newerStart.Add(45*time.Minute),
			2700,
		); err != nil {
			t.Fatalf("插入新游玩记录失败: %v", err)
		}

		resp, err := newService.GetGames(vo.GameListRequest{})
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(resp.Games) != 1 {
			t.Fatalf("期望获取 1 个游戏, 实际获取 %d 个", len(resp.Games))
		}
		if resp.Games[0].LastPlayedAt == nil {
			t.Fatal("期望返回最近游玩时间，但得到 nil")
		}
		if resp.Games[0].LastPlayedAt.Sub(newerStart).Abs() > time.Millisecond {
			t.Errorf("最近游玩时间不正确: 期望 %v, 得到 %v", newerStart, *resp.Games[0].LastPlayedAt)
		}
	})

	t.Run("分页搜索和 has_more", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		for i, name := range []string{"Alpha One", "Alpha Two", "Beta"} {
			game := createTestGame()
			game.ID = fmt.Sprintf("paged-%d", i)
			game.Name = name
			game.Company = "Studio"
			if err := addGameViaMetadata(newService, game); err != nil {
				t.Fatalf("添加游戏失败: %v", err)
			}
		}

		resp, err := newService.GetGames(vo.GameListRequest{
			Limit:       1,
			SearchQuery: "alpha",
			SortBy:      enums.GameListSortByName,
			SortOrder:   enums.SortOrderAsc,
		})
		if err != nil {
			t.Fatalf("分页查询失败: %v", err)
		}
		if len(resp.Games) != 1 {
			t.Fatalf("期望返回 1 个游戏, 实际 %d", len(resp.Games))
		}
		if resp.Total != 2 {
			t.Fatalf("期望 total 为 2, 实际 %d", resp.Total)
		}
		if !resp.HasMore {
			t.Fatal("期望 has_more 为 true")
		}
		if resp.Games[0].Name != "Alpha One" {
			t.Fatalf("期望按名称升序返回 Alpha One, 实际 %s", resp.Games[0].Name)
		}
	})

	t.Run("别名参与搜索", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		game := createTestGame()
		game.ID = "alias-search"
		game.Name = "素晴らしき日々～不連続存在～"
		game.Aliases = []string{" 素晴日 ", "SubaHibi", "subahibi"}
		if err := addGameViaMetadata(newService, game); err != nil {
			t.Fatalf("添加带别名的游戏失败: %v", err)
		}

		resp, err := newService.GetGames(vo.GameListRequest{SearchQuery: "素晴日"})
		if err != nil {
			t.Fatalf("按别名搜索失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Games) != 1 || resp.Games[0].ID != game.ID {
			t.Fatalf("别名搜索结果异常: total=%d games=%+v", resp.Total, resp.Games)
		}
		wantAliases := []string{"素晴日", "SubaHibi"}
		if !reflect.DeepEqual(resp.Games[0].Aliases, wantAliases) {
			t.Fatalf("别名归一化异常: got=%#v want=%#v", resp.Games[0].Aliases, wantAliases)
		}
	})

	t.Run("发售日期排序空日期始终在最后", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		fixtures := []struct {
			id          string
			name        string
			releaseDate string
		}{
			{id: "release-empty", name: "No Date", releaseDate: ""},
			{id: "release-newer", name: "Newer", releaseDate: "2025-06-01"},
			{id: "release-older", name: "Older", releaseDate: "2024-01-02"},
		}
		for _, item := range fixtures {
			game := createTestGame()
			game.ID = item.id
			game.Name = item.name
			game.CoverURL = ""
			game.ReleaseDate = item.releaseDate
			if err := addGameViaMetadata(newService, game); err != nil {
				t.Fatalf("添加游戏 %s 失败: %v", item.id, err)
			}
		}

		ascResp, err := newService.GetGames(vo.GameListRequest{
			SortBy:    enums.GameListSortByReleaseDate,
			SortOrder: enums.SortOrderAsc,
		})
		if err != nil {
			t.Fatalf("按发售日期升序查询失败: %v", err)
		}
		assertGameOrder(t, ascResp.Games, []string{"release-older", "release-newer", "release-empty"})

		descResp, err := newService.GetGames(vo.GameListRequest{
			SortBy:    enums.GameListSortByReleaseDate,
			SortOrder: enums.SortOrderDesc,
		})
		if err != nil {
			t.Fatalf("按发售日期降序查询失败: %v", err)
		}
		assertGameOrder(t, descResp.Games, []string{"release-newer", "release-older", "release-empty"})
	})

	t.Run("状态反选排除指定状态", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		fixtures := []struct {
			id     string
			name   string
			status enums.GameStatus
		}{
			{id: "status-not-started", name: "Alpha", status: enums.StatusNotStarted},
			{id: "status-playing", name: "Beta", status: enums.StatusPlaying},
			{id: "status-completed", name: "Gamma", status: enums.StatusCompleted},
		}
		for _, item := range fixtures {
			game := createTestGame()
			game.ID = item.id
			game.Name = item.name
			game.Status = item.status
			if err := addGameViaMetadata(newService, game); err != nil {
				t.Fatalf("添加游戏 %s 失败: %v", item.id, err)
			}
		}

		status := enums.StatusPlaying
		resp, err := newService.GetGames(vo.GameListRequest{
			Status:        &status,
			ExcludeStatus: true,
			SortBy:        enums.GameListSortByName,
			SortOrder:     enums.SortOrderAsc,
		})
		if err != nil {
			t.Fatalf("状态反选查询失败: %v", err)
		}
		if resp.Total != 2 {
			t.Fatalf("期望 total 为 2, 实际 %d", resp.Total)
		}
		assertGameOrder(t, resp.Games, []string{"status-not-started", "status-completed"})
	})

	t.Run("标签反选排除带任一指定标签的游戏", func(t *testing.T) {
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		for _, item := range []struct {
			id   string
			name string
			tags []string
		}{
			{id: "tag-action", name: "Action Only", tags: []string{"Action"}},
			{id: "tag-drama", name: "Drama Only", tags: []string{"Drama"}},
			{id: "tag-both", name: "Both", tags: []string{"Action", "Drama"}},
			{id: "tag-other", name: "Other", tags: []string{"Comedy"}},
		} {
			game := createTestGame()
			game.ID = item.id
			game.Name = item.name
			if err := addGameViaMetadata(newService, game); err != nil {
				t.Fatalf("添加游戏 %s 失败: %v", item.id, err)
			}
			for _, tag := range item.tags {
				if _, err := newDB.Exec(
					`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, 'user', 1, FALSE, ?, ?)`,
					fmt.Sprintf("%s-%s", item.id, tag),
					item.id,
					tag,
					time.Now(),
					time.Now(),
				); err != nil {
					t.Fatalf("添加标签 %s/%s 失败: %v", item.id, tag, err)
				}
			}
		}

		resp, err := newService.GetGames(vo.GameListRequest{
			Tags:        []string{"Action", "Drama"},
			ExcludeTags: true,
			SortBy:      enums.GameListSortByName,
			SortOrder:   enums.SortOrderAsc,
		})
		if err != nil {
			t.Fatalf("标签反选查询失败: %v", err)
		}
		if resp.Total != 1 {
			t.Fatalf("期望 total 为 1, 实际 %d", resp.Total)
		}
		assertGameOrder(t, resp.Games, []string{"tag-other"})
	})
}

func assertGameOrder(t *testing.T, games []models.Game, wantIDs []string) {
	t.Helper()
	if len(games) != len(wantIDs) {
		t.Fatalf("期望返回 %d 个游戏, 实际 %d", len(wantIDs), len(games))
	}
	for i, wantID := range wantIDs {
		if games[i].ID != wantID {
			t.Fatalf("第 %d 个游戏期望 %s, 实际 %s", i, wantID, games[i].ID)
		}
	}
}

func TestGameService_UpdateGame(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功更新游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "update-test-001"

		// 先添加游戏
		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 更新游戏信息
		game.Name = "更新后的游戏名称"
		game.Company = "更新后的公司"
		game.Summary = "更新后的简介"

		err = gameService.UpdateGame(game)
		if err != nil {
			t.Fatalf("更新游戏失败: %v", err)
		}

		// 验证更新
		updatedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Fatalf("获取游戏失败: %v", err)
		}

		if updatedGame.Name != "更新后的游戏名称" {
			t.Errorf("游戏名称未更新: 期望 %s, 得到 %s", "更新后的游戏名称", updatedGame.Name)
		}
		if updatedGame.Company != "更新后的公司" {
			t.Errorf("公司名称未更新: 期望 %s, 得到 %s", "更新后的公司", updatedGame.Company)
		}
		if updatedGame.Summary != "更新后的简介" {
			t.Errorf("简介未更新: 期望 %s, 得到 %s", "更新后的简介", updatedGame.Summary)
		}
	})

	t.Run("更新不存在的游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "non-existent-id"

		err := gameService.UpdateGame(game)
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
	})
}

func TestGameService_UpdateGameFromRemoteRespectsMetadataLock(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{
		MetadataSources: []string{string(enums.Bangumi)},
	})

	game := createTestGame()
	game.ID = "locked-remote-update-001"
	game.SourceType = enums.Bangumi
	game.SourceID = "12345"
	game.MetadataLocked = true
	if err := addGameViaMetadata(gameService, game); err != nil {
		t.Fatalf("添加游戏失败: %v", err)
	}

	err := gameService.UpdateGameFromRemote(game.ID)
	if err == nil {
		t.Fatal("期望锁定游戏远端更新返回错误，但没有错误")
	}
	if !strings.Contains(err.Error(), "元数据已锁定") {
		t.Fatalf("期望元数据锁定错误，实际: %v", err)
	}
}

func TestGameService_DeleteGame(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功删除游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "delete-test-001"

		// 先添加游戏
		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 删除游戏
		err = gameService.DeleteGame(game.ID)
		if err != nil {
			t.Fatalf("删除游戏失败: %v", err)
		}

		// 验证游戏已删除
		_, err = gameService.GetGameByID(game.ID)
		if err == nil {
			t.Error("游戏应该已被删除，但仍然可以获取")
		}
	})

	t.Run("删除不存在的游戏", func(t *testing.T) {
		err := gameService.DeleteGame("non-existent-id")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
	})

	t.Run("删除带分类的游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "delete-test-002"

		// 添加游戏
		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 添加游戏分类关系
		_, err = db.Exec("INSERT INTO game_categories (game_id, category_id) VALUES (?, ?)",
			game.ID, "category-001")
		if err != nil {
			t.Fatalf("添加游戏分类失败: %v", err)
		}

		// 删除游戏（应该级联删除分类关系）
		err = gameService.DeleteGame(game.ID)
		if err != nil {
			t.Fatalf("删除游戏失败: %v", err)
		}

		// 验证分类关系已删除
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM game_categories WHERE game_id = ?", game.ID).Scan(&count)
		if err != nil {
			t.Fatalf("查询分类关系失败: %v", err)
		}

		if count != 0 {
			t.Errorf("期望分类关系已删除，但还有 %d 条记录", count)
		}
	})
}

func TestGameService_DeleteGames(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("批量删除游戏", func(t *testing.T) {
		game1 := createTestGame()
		game1.ID = "batch-del-001"
		game2 := createTestGame()
		game2.ID = "batch-del-002"

		if err := addGameViaMetadata(gameService, game1); err != nil {
			t.Fatalf("添加游戏1失败: %v", err)
		}
		if err := addGameViaMetadata(gameService, game2); err != nil {
			t.Fatalf("添加游戏2失败: %v", err)
		}

		// 添加关联数据
		if _, err := db.Exec("INSERT INTO game_categories (game_id, category_id) VALUES (?, ?)", game1.ID, "category-001"); err != nil {
			t.Fatalf("添加游戏分类失败: %v", err)
		}
		if _, err := db.Exec("INSERT INTO game_categories (game_id, category_id) VALUES (?, ?)", game2.ID, "category-002"); err != nil {
			t.Fatalf("添加游戏分类失败: %v", err)
		}
		if _, err := db.Exec("INSERT INTO play_sessions (id, game_id, start_time, end_time, duration) VALUES (?, ?, ?, ?, ?)",
			"ps-001", game1.ID, time.Now(), time.Now(), 120); err != nil {
			t.Fatalf("添加游玩会话失败: %v", err)
		}

		if err := gameService.DeleteGames([]string{game1.ID, game2.ID}); err != nil {
			t.Fatalf("批量删除失败: %v", err)
		}

		// 验证游戏已删除
		if _, err := gameService.GetGameByID(game1.ID); err == nil {
			t.Error("游戏1应该已被删除")
		}
		if _, err := gameService.GetGameByID(game2.ID); err == nil {
			t.Error("游戏2应该已被删除")
		}

		// 验证关联已删除
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM game_categories WHERE game_id IN (?, ?)", game1.ID, game2.ID).Scan(&count); err != nil {
			t.Fatalf("查询分类关系失败: %v", err)
		}
		if count != 0 {
			t.Errorf("分类关系未清理，剩余 %d 条", count)
		}

		if err := db.QueryRow("SELECT COUNT(*) FROM play_sessions WHERE game_id IN (?, ?)", game1.ID, game2.ID).Scan(&count); err != nil {
			t.Fatalf("查询游玩会话失败: %v", err)
		}
		if count != 0 {
			t.Errorf("游玩会话未清理，剩余 %d 条", count)
		}
	})
}

func TestGameService_CompleteWorkflow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("完整的CRUD流程", func(t *testing.T) {
		// 1. 添加游戏
		game := createTestGame()
		game.ID = "workflow-test-001"

		err := addGameViaMetadata(gameService, game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 2. 获取单个游戏
		savedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Fatalf("获取游戏失败: %v", err)
		}
		if savedGame.Name != game.Name {
			t.Errorf("游戏名称不匹配")
		}

		// 3. 获取所有游戏
		resp, err := gameService.GetGames(vo.GameListRequest{})
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}
		if len(resp.Games) == 0 {
			t.Error("游戏列表为空")
		}

		// 4. 更新游戏
		savedGame.Name = "更新后的名称"
		err = gameService.UpdateGame(savedGame)
		if err != nil {
			t.Fatalf("更新游戏失败: %v", err)
		}

		// 5. 验证更新
		updatedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Fatalf("获取更新后的游戏失败: %v", err)
		}
		if updatedGame.Name != "更新后的名称" {
			t.Error("游戏名称未更新")
		}

		// 6. 删除游戏
		err = gameService.DeleteGame(game.ID)
		if err != nil {
			t.Fatalf("删除游戏失败: %v", err)
		}

		// 7. 验证删除
		_, err = gameService.GetGameByID(game.ID)
		if err == nil {
			t.Error("游戏应该已被删除")
		}
	})
}
