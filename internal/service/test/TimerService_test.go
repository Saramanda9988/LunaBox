package service

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	service2 "lunabox/internal/service"
	"testing"
	"time"
)

func TestTimerService_ReportPlaySession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	service := service2.NewTimerService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("成功上报游玩记录", func(t *testing.T) {
		session := models.PlaySession{
			ID:        "session-001",
			UserID:    "user-001",
			GameID:    "game-001",
			StartTime: time.Now().Add(-2 * time.Hour),
			EndTime:   time.Now(),
			Duration:  7200, // 2 hours in seconds
		}

		err := service.ReportPlaySession(session)
		if err != nil {
			t.Fatalf("上报游玩记录失败: %v", err)
		}

		// 验证记录已保存
		sessions, err := service.GetPlaySessions(session.UserID)
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 1 {
			t.Errorf("期望 1 条记录，实际获取 %d 条", len(sessions))
		}

		if sessions[0].Duration != 7200 {
			t.Errorf("时长不匹配: 期望 7200, 得到 %d", sessions[0].Duration)
		}
	})

	t.Run("自动生成ID", func(t *testing.T) {
		session := models.PlaySession{
			ID:        "", // 不提供ID
			UserID:    "user-002",
			GameID:    "game-002",
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now(),
			Duration:  3600,
		}

		err := service.ReportPlaySession(session)
		if err != nil {
			t.Fatalf("上报游玩记录失败: %v", err)
		}

		sessions, err := service.GetPlaySessions(session.UserID)
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 1 {
			t.Error("未找到上报的记录")
		}
	})

	t.Run("自动计算时长", func(t *testing.T) {
		startTime := time.Now().Add(-30 * time.Minute)
		endTime := time.Now()

		session := models.PlaySession{
			UserID:    "user-003",
			GameID:    "game-003",
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  0, // 不提供时长
		}

		err := service.ReportPlaySession(session)
		if err != nil {
			t.Fatalf("上报游玩记录失败: %v", err)
		}

		sessions, err := service.GetPlaySessions(session.UserID)
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 1 {
			t.Fatal("未找到上报的记录")
		}

		// 时长应该约等于 1800 秒（30分钟）
		if sessions[0].Duration < 1790 || sessions[0].Duration > 1810 {
			t.Errorf("自动计算的时长不正确: %d 秒", sessions[0].Duration)
		}
	})
}

func TestTimerService_GetPlaySessions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	service := service2.NewTimerService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("获取用户的所有游玩记录", func(t *testing.T) {
		userID := "user-multi"

		// 添加多条记录
		for i := 1; i <= 3; i++ {
			session := models.PlaySession{
				UserID:    userID,
				GameID:    "game-001",
				StartTime: time.Now().Add(-time.Duration(i) * time.Hour),
				EndTime:   time.Now(),
				Duration:  3600 * i,
			}
			err := service.ReportPlaySession(session)
			if err != nil {
				t.Fatalf("添加记录 %d 失败: %v", i, err)
			}
		}

		sessions, err := service.GetPlaySessions(userID)
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 3 {
			t.Errorf("期望 3 条记录，实际获取 %d 条", len(sessions))
		}
	})

	t.Run("用户无记录", func(t *testing.T) {
		sessions, err := service.GetPlaySessions("non-existent-user")
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 0 {
			t.Errorf("期望空列表，实际获取 %d 条记录", len(sessions))
		}
	})
}

func TestTimerService_GetPlaySessionsByGameID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	service := service2.NewTimerService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("获取指定游戏的游玩记录", func(t *testing.T) {
		gameID := "game-target"

		// 添加目标游戏的记录
		for i := 1; i <= 2; i++ {
			session := models.PlaySession{
				UserID:    "user-001",
				GameID:    gameID,
				StartTime: time.Now().Add(-time.Duration(i) * time.Hour),
				EndTime:   time.Now(),
				Duration:  1800,
			}
			err := service.ReportPlaySession(session)
			if err != nil {
				t.Fatalf("添加记录失败: %v", err)
			}
		}

		// 添加其他游戏的记录
		otherSession := models.PlaySession{
			UserID:    "user-001",
			GameID:    "game-other",
			StartTime: time.Now(),
			EndTime:   time.Now(),
			Duration:  1000,
		}
		_ = service.ReportPlaySession(otherSession)

		// 获取目标游戏的记录
		sessions, err := service.GetPlaySessionsByGameID(gameID)
		if err != nil {
			t.Fatalf("获取游玩记录失败: %v", err)
		}

		if len(sessions) != 2 {
			t.Errorf("期望 2 条记录，实际获取 %d 条", len(sessions))
		}

		for _, s := range sessions {
			if s.GameID != gameID {
				t.Errorf("记录的 GameID 不匹配: 期望 %s, 得到 %s", gameID, s.GameID)
			}
		}
	})
}

func TestTimerService_GetTotalPlayTime(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	service := service2.NewTimerService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("计算总游玩时长", func(t *testing.T) {
		gameID := "game-total"

		// 添加多条记录
		durations := []int{1800, 3600, 7200} // 0.5h, 1h, 2h
		expectedTotal := 0

		for _, duration := range durations {
			session := models.PlaySession{
				UserID:    "user-001",
				GameID:    gameID,
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Duration:  duration,
			}
			err := service.ReportPlaySession(session)
			if err != nil {
				t.Fatalf("添加记录失败: %v", err)
			}
			expectedTotal += duration
		}

		// 获取总时长
		totalTime, err := service.GetTotalPlayTime(gameID)
		if err != nil {
			t.Fatalf("获取总时长失败: %v", err)
		}

		if totalTime != expectedTotal {
			t.Errorf("总时长不匹配: 期望 %d, 得到 %d", expectedTotal, totalTime)
		}
	})

	t.Run("游戏无记录时返回0", func(t *testing.T) {
		totalTime, err := service.GetTotalPlayTime("non-existent-game")
		if err != nil {
			t.Fatalf("获取总时长失败: %v", err)
		}

		if totalTime != 0 {
			t.Errorf("期望 0, 得到 %d", totalTime)
		}
	})
}

func TestTimerService_StartGameWithTracking_GetPath(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 先添加一个游戏到数据库
	gameService := service2.NewGameService()
	gameService.Init(context.Background(), db, &appconf.AppConfig{})

	game := models.Game{
		ID:     "game-with-path",
		UserID: "user-001",
		Name:   "测试游戏",
		Path:   "C:\\TestGame\\game.exe",
	}
	err := gameService.AddGame(game)
	if err != nil {
		t.Fatalf("添加游戏失败: %v", err)
	}

	timerService := service2.NewTimerService()
	timerService.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("获取游戏路径", func(t *testing.T) {
		path, err := timerService.GetGamePath(game.ID)
		if err != nil {
			t.Fatalf("获取游戏路径失败: %v", err)
		}

		if path != game.Path {
			t.Errorf("路径不匹配: 期望 %s, 得到 %s", game.Path, path)
		}
	})

	t.Run("游戏不存在", func(t *testing.T) {
		_, err := timerService.GetGamePath("non-existent-game")
		if err == nil {
			t.Error("期望返回错误，但没有错误")
		}
	})
}

func TestTimerService_CompleteWorkflow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	service := service2.NewTimerService()
	service.Init(context.Background(), db, &appconf.AppConfig{})

	t.Run("完整的游玩记录流程", func(t *testing.T) {
		userID := "workflow-user"
		gameID := "workflow-game"

		// 1. 上报游玩记录
		session := models.PlaySession{
			UserID:    userID,
			GameID:    gameID,
			StartTime: time.Now().Add(-1 * time.Hour),
			EndTime:   time.Now(),
			Duration:  3600,
		}

		err := service.ReportPlaySession(session)
		if err != nil {
			t.Fatalf("上报记录失败: %v", err)
		}

		// 2. 获取用户的所有记录
		sessions, err := service.GetPlaySessions(userID)
		if err != nil {
			t.Fatalf("获取用户记录失败: %v", err)
		}
		if len(sessions) != 1 {
			t.Error("用户记录数量不正确")
		}

		// 3. 获取游戏的所有记录
		gameSessions, err := service.GetPlaySessionsByGameID(gameID)
		if err != nil {
			t.Fatalf("获取游戏记录失败: %v", err)
		}
		if len(gameSessions) != 1 {
			t.Error("游戏记录数量不正确")
		}

		// 4. 获取总游玩时长
		totalTime, err := service.GetTotalPlayTime(gameID)
		if err != nil {
			t.Fatalf("获取总时长失败: %v", err)
		}
		if totalTime != 3600 {
			t.Errorf("总时长不正确: 期望 3600, 得到 %d", totalTime)
		}

		// 5. 再添加一条记录
		session2 := models.PlaySession{
			UserID:    userID,
			GameID:    gameID,
			StartTime: time.Now().Add(-30 * time.Minute),
			EndTime:   time.Now(),
			Duration:  1800,
		}
		err = service.ReportPlaySession(session2)
		if err != nil {
			t.Fatalf("添加第二条记录失败: %v", err)
		}

		// 6. 验证总时长更新
		totalTime, err = service.GetTotalPlayTime(gameID)
		if err != nil {
			t.Fatalf("获取总时长失败: %v", err)
		}
		if totalTime != 5400 {
			t.Errorf("总时长不正确: 期望 5400, 得到 %d", totalTime)
		}
	})
}
