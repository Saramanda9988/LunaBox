package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"

	"lunabox/backend/appconf"
	"lunabox/backend/applog"
	"lunabox/backend/migrations"
	"lunabox/backend/service"
	"lunabox/backend/utils/dbutils"
	_ "lunabox/backend/utils/dbutils/duckdbdriver" // DuckDB 驱动与 Appender
)

// duckDBThreads 是 DuckDB 的工作线程数，见 OpenDatabase。
var duckDBThreads = max(1, min(4, runtime.NumCPU()))

// RunPendingRestores 执行设置页安排的恢复：先恢复完整数据，再恢复数据库，都要在打开数据库之前。
// 开始前先还原上次替换到一半就退出的恢复（service.RecoverInterruptedRestore）。
// 有恢复要执行时把各阶段的进度交给 progress（可为 nil），全部成功后再报告一次 service.RestoreStageDone。
// 恢复是原子的：失败时现有数据保持不变，返回的错误由调用方显示在启动错误窗里，不再继续启动。
func RunPendingRestores(config *appconf.AppConfig, logger *applog.FileLogger, progress service.RestoreProgressFunc) error {
	rolledBack, err := service.RecoverInterruptedRestore()
	if err != nil {
		logger.Error("rolling back interrupted restore failed: " + err.Error())
		return err
	}
	if rolledBack {
		logger.Warning("rolled back a restore that was interrupted while replacing data")
	}
	if config.PendingFullRestore == "" && config.PendingDBRestore == "" {
		return nil
	}
	if config.PendingFullRestore != "" {
		if _, err := service.ExecuteFullDataRestore(config, progress); err != nil {
			logger.Error("full data restore failed: " + err.Error())
			return err
		}
		logger.Info("full data restore completed")
	}
	if config.PendingDBRestore != "" {
		if _, err := service.ExecuteDBRestore(config, progress); err != nil {
			logger.Error("database restore failed: " + err.Error())
			return err
		}
		logger.Info("database restore completed")
	}
	if progress != nil {
		progress(service.RestoreProgress{Stage: service.RestoreStageDone})
	}
	return nil
}

// OpenDatabase 打开数据库（WAL 损坏时恢复）并设置时区，再初始化结构、执行迁移与索引。
// 初始化或迁移失败时也返回已打开的数据库，由调用方在退出时关闭。
func OpenDatabase(ctx context.Context, path, timeZone string, logger *applog.FileLogger) (*sql.DB, error) {
	db, err := dbutils.OpenDuckDBWithWALRecovery(ctx, path, logger)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	if _, err = db.Exec("SET GLOBAL checkpoint_threshold = '4 MiB'"); err != nil {
		logger.Warning("Failed to set DuckDB automatic checkpoint threshold; using the default: " + err.Error())
	}
	// DuckDB 默认按 CPU 核数开线程，每个线程各自申请执行缓冲。桌面端的库只有几十 MB，多线程不会更快，
	// 用完释放的内存却留在 Windows 进程堆里不还给系统：20 核的机器上每进一次首页（全局统计）NT 堆就涨几 MB，
	// 反复切换页面能涨到一两百 MB。4 个线程时堆只有原来的约 1/4，统计查询还快了 20%。
	if _, err = db.Exec(fmt.Sprintf("SET GLOBAL threads = %d", duckDBThreads)); err != nil {
		logger.Warning("Failed to limit DuckDB threads; using the default: " + err.Error())
	}
	if timeZone == "" {
		timeZone = "UTC"
	}
	if _, err = db.Exec(fmt.Sprintf("SET TimeZone = '%s'", timeZone)); err != nil {
		logger.Warning("Failed to set timezone: " + err.Error())
	}

	if err := migrations.InitSchema(db); err != nil {
		return db, fmt.Errorf("初始化数据库结构失败: %w", err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		return db, fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := migrations.InitIndexes(db); err != nil {
		return db, fmt.Errorf("初始化数据库索引失败: %w", err)
	}
	if err := dbutils.CheckpointDuckDB(ctx, db); err != nil {
		logger.Warning("Database checkpoint after schema initialization failed; committed changes remain in WAL: " + err.Error())
	}
	return db, nil
}
