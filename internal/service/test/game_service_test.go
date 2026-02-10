package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/service"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

// createTestGame 创建测试游戏数据
func createTestGame() models.Game {
	return models.Game{
		ID:         "test-game-001",
		Name:       "测试游戏",
		CoverURL:   "https://example.com/cover.jpg",
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

		err := gameService.AddGame(game)
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

		err := gameService.AddGame(game)
		if err != nil {
			t.Fatalf("添加游戏失败: %v", err)
		}

		// 验证至少有一个游戏被添加（由于ID是自动生成的，无法直接验证）
		games, err := gameService.GetGames()
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(games) < 1 {
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
		err := gameService.AddGame(game)
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
			err := gameService.AddGame(game)
			if err != nil {
				t.Fatalf("添加游戏 %d 失败: %v", i, err)
			}
		}

		games, err := gameService.GetGames()
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(games) != 3 {
			t.Errorf("期望获取 3 个游戏, 实际获取 %d 个", len(games))
		}
	})

	t.Run("空列表", func(t *testing.T) {
		// 使用新的数据库
		newDB, newCleanup := setupTestDB(t)
		defer newCleanup()

		newService := service.NewGameService()
		newService.Init(context.Background(), newDB, &appconf.AppConfig{})

		games, err := newService.GetGames()
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}

		if len(games) != 0 {
			t.Errorf("期望空列表, 实际获取 %d 个游戏", len(games))
		}
	})
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
		err := gameService.AddGame(game)
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

func TestGameService_DeleteGame(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功删除游戏", func(t *testing.T) {
		game := createTestGame()
		game.ID = "delete-test-001"

		// 先添加游戏
		err := gameService.AddGame(game)
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
		err := gameService.AddGame(game)
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

		if err := gameService.AddGame(game1); err != nil {
			t.Fatalf("添加游戏1失败: %v", err)
		}
		if err := gameService.AddGame(game2); err != nil {
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

		err := gameService.AddGame(game)
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
		games, err := gameService.GetGames()
		if err != nil {
			t.Fatalf("获取游戏列表失败: %v", err)
		}
		if len(games) == 0 {
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
