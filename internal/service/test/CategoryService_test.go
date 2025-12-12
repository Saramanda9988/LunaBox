package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
)

func TestCategoryService_Init(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	// 验证系统分类是否自动创建
	categories, err := categoryService.GetCategories()
	if err != nil {
		t.Fatalf("获取分类失败: %v", err)
	}

	foundSystemCategory := false
	for _, c := range categories {
		if c.Name == "最喜欢的游戏" && c.IsSystem {
			foundSystemCategory = true
			break
		}
	}

	if !foundSystemCategory {
		t.Error("初始化时未创建系统分类 '最喜欢的游戏'")
	}
}

func TestCategoryService_AddCategory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功添加分类", func(t *testing.T) {
		err := categoryService.AddCategory("测试分类")
		if err != nil {
			t.Fatalf("添加分类失败: %v", err)
		}

		categories, err := categoryService.GetCategories()
		if err != nil {
			t.Fatalf("获取分类失败: %v", err)
		}

		found := false
		for _, c := range categories {
			if c.Name == "测试分类" {
				found = true
				if c.IsSystem {
					t.Error("新添加的分类不应为系统分类")
				}
				break
			}
		}

		if !found {
			t.Error("未找到新添加的分类")
		}
	})
}

func TestCategoryService_DeleteCategory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	// 添加一个普通分类
	err := categoryService.AddCategory("待删除分类")
	if err != nil {
		t.Fatalf("添加分类失败: %v", err)
	}

	categories, err := categoryService.GetCategories()
	if err != nil {
		t.Fatalf("获取分类失败: %v", err)
	}

	var targetID string
	var systemID string
	for _, c := range categories {
		if c.Name == "待删除分类" {
			targetID = c.ID
		}
		if c.IsSystem {
			systemID = c.ID
		}
	}

	t.Run("成功删除普通分类", func(t *testing.T) {
		err := categoryService.DeleteCategory(targetID)
		if err != nil {
			t.Fatalf("删除分类失败: %v", err)
		}

		// 验证已删除
		cats, _ := categoryService.GetCategories()
		for _, c := range cats {
			if c.ID == targetID {
				t.Error("分类未被删除")
			}
		}
	})

	t.Run("禁止删除系统分类", func(t *testing.T) {
		if systemID == "" {
			t.Skip("未找到系统分类，跳过测试")
		}
		err := categoryService.DeleteCategory(systemID)
		if err == nil {
			t.Error("期望删除系统分类失败，但成功了")
		}
	})
}

func TestCategoryService_GameCategoryRelation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	// 准备数据
	game := createTestGame()
	game.ID = "game-rel-001"
	if err := gameService.AddGame(game); err != nil {
		t.Fatalf("添加游戏失败: %v", err)
	}

	if err := categoryService.AddCategory("游戏分类"); err != nil {
		t.Fatalf("添加分类失败: %v", err)
	}

	categories, _ := categoryService.GetCategories()
	var categoryID string
	for _, c := range categories {
		if c.Name == "游戏分类" {
			categoryID = c.ID
			break
		}
	}

	t.Run("添加游戏到分类", func(t *testing.T) {
		err := categoryService.AddGameToCategory(game.ID, categoryID)
		if err != nil {
			t.Fatalf("添加游戏到分类失败: %v", err)
		}

		// 验证游戏数量
		cats, _ := categoryService.GetCategories()
		for _, c := range cats {
			if c.ID == categoryID {
				if c.GameCount != 1 {
					t.Errorf("期望游戏数量为 1，实际为 %d", c.GameCount)
				}
			}
		}
	})

	t.Run("从分类移除游戏", func(t *testing.T) {
		err := categoryService.RemoveGameFromCategory(game.ID, categoryID)
		if err != nil {
			t.Fatalf("从分类移除游戏失败: %v", err)
		}

		// 验证游戏数量
		cats, _ := categoryService.GetCategories()
		for _, c := range cats {
			if c.ID == categoryID {
				if c.GameCount != 0 {
					t.Errorf("期望游戏数量为 0，实际为 %d", c.GameCount)
				}
			}
		}
	})
}

func TestCategoryService_DeleteCategoryWithGames(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	categoryService := service.NewCategoryService()
	categoryService.Init(context.Background(), db, &appconf.AppConfig{})

	gameService := service.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	// 准备数据
	game := createTestGame()
	game.ID = "game-del-cat-001"
	gameService.AddGame(game)

	categoryService.AddCategory("关联分类")
	categories, _ := categoryService.GetCategories()
	var categoryID string
	for _, c := range categories {
		if c.Name == "关联分类" {
			categoryID = c.ID
			break
		}
	}

	// 建立关联
	categoryService.AddGameToCategory(game.ID, categoryID)

	t.Run("删除分类级联删除关联", func(t *testing.T) {
		err := categoryService.DeleteCategory(categoryID)
		if err != nil {
			t.Fatalf("删除分类失败: %v", err)
		}

		// 验证关联表数据已清理
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM game_categories WHERE category_id = ?", categoryID).Scan(&count)
		if err != nil {
			t.Fatalf("查询关联表失败: %v", err)
		}
		if count != 0 {
			t.Errorf("分类删除后关联数据未清理，剩余 %d 条", count)
		}

		// 验证游戏本身未被删除
		savedGame, err := gameService.GetGameByID(game.ID)
		if err != nil {
			t.Errorf("游戏不应被删除")
		}
		if savedGame.ID != game.ID {
			t.Errorf("获取到的游戏ID不匹配")
		}
	})
}
