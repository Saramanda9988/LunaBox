package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTimerService_ReportPlaySession(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	timerService := service.NewTimerService()
	timerService.Init(context.Background(), db, &appconf.AppConfig{})
}

// TestTimerService_Integration_RealProcess 测试真实的进程启动和计时
func TestTimerService_Integration_RealProcess(t *testing.T) {
	// 1. 准备环境
	// 获取当前测试文件所在目录
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	testDir := wd

	srcPath := filepath.Join(testDir, "testdata", "test_for_timer.go")
	exePath := filepath.Join(testDir, "testdata", "test_game.exe")

	// 2. 编译测试用的"游戏"程序
	t.Logf("Compiling %s to %s...", srcPath, exePath)
	buildCmd := exec.Command("go", "build", "-o", exePath, srcPath)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile test game: %v\nOutput: %s", err, out)
	}
	// 测试结束后清理生成的 exe
	defer os.Remove(exePath)

	// 3. 初始化数据库
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 4. 插入测试游戏记录
	gameID := "integration-test-game"
	_, err = db.Exec("INSERT INTO games (id, path) VALUES (?, ?)", gameID, exePath)
	if err != nil {
		t.Fatalf("Failed to insert game: %v", err)
	}

	// 5. 初始化 TimerService
	svc := service.NewTimerService()
	svc.Init(context.Background(), db, &appconf.AppConfig{})

	// 6. 启动游戏（开始计时）
	t.Log("Starting game with tracking...")
	userID := "test-user"
	err = svc.StartGameWithTracking(userID, gameID)
	if err != nil {
		t.Fatalf("StartGameWithTracking failed: %v", err)
	}

	// 7. 等待游戏结束
	// 我们的测试程序设定为运行 2 秒，我们等待 4 秒以确保它完全退出并更新数据库
	t.Log("Waiting for game to finish (approx 2s)...")
	time.Sleep(4 * time.Second)

	// 8. 验证结果
	var duration int
	var startTime, endTime time.Time
	err = db.QueryRow(`
		SELECT duration, start_time, end_time 
		FROM play_sessions 
		WHERE game_id = ?`, gameID).Scan(&duration, &startTime, &endTime)

	if err != nil {
		t.Fatalf("Failed to query session: %v", err)
	}

	t.Logf("Game finished. Recorded duration: %d seconds", duration)
	t.Logf("Start: %v, End: %v", startTime, endTime)

	// 验证时长是否合理 (应该在 2 秒左右)
	// 考虑到启动开销和系统调度，允许 1-5 秒的范围
	if duration < 1 {
		t.Errorf("Duration too short. Expected >= 1, got %d", duration)
	}
	if duration > 5 {
		t.Errorf("Duration too long. Expected <= 5, got %d", duration)
	}

	if endTime.Before(startTime) {
		t.Error("End time is before start time")
	}
}
