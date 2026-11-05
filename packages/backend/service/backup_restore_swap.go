package service

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"lunabox/backend/utils/apputils"
)

// 启动时的恢复分两步，保证要么全部生效、要么现有数据保持不变：
//
//  1. 准备：把备份解压到数据目录下的暂存目录（restoreStagingDirName），在那里导入出新的数据库。这一步不碰现有数据。
//  2. 替换（commitRestoreSwap）：先把替换计划写进恢复日志（restoreJournalName），再逐项把现有的文件或目录移到
//     回滚目录、把暂存的移到原位置。暂存目录与原位置在同一个目录下，替换都是重命名，很快。某一项失败时按相反
//     顺序移回去；替换到一半进程退出（断电、被结束）时，下次启动由 RecoverInterruptedRestore 按日志移回去。
const (
	restoreStagingDirName   = ".restore-staging"
	restoreRollbackDirName  = ".restore-rollback"
	restoreJournalName      = ".restore-journal.json"
	restoreStagedConfigName = ".restore-staging-appconf.json"  // 暂存的设置文件，在配置目录下
	restoreConfigBackupName = ".restore-rollback-appconf.json" // 替换前的设置文件，在配置目录下
)

// restoreSwapItem 是一项替换：把 Live 换成 Staged（为空时只移走 Live，如旧数据库的 .wal），原有的移到 Backup。
type restoreSwapItem struct {
	Live    string `json:"live"`
	Staged  string `json:"staged,omitempty"`
	Backup  string `json:"backup"`
	Existed bool   `json:"existed"` // 开始替换时 Live 是否存在
}

// restoreRollbackError 表示替换失败后没能全部移回去：恢复日志保留在磁盘上，下次启动再回滚。
type restoreRollbackError struct {
	err, rollbackErr error
}

func (e *restoreRollbackError) Error() string {
	return fmt.Sprintf("%v；还原现有数据失败: %v", e.err, e.rollbackErr)
}

func (e *restoreRollbackError) Unwrap() error { return e.err }

// restoreFailure 给恢复失败的错误加上现有数据是否保持不变的说明，显示在启动错误窗里。
func restoreFailure(what string, err error) error {
	var rb *restoreRollbackError
	if errors.As(err, &rb) {
		return fmt.Errorf("%s失败，且未能还原现有数据，下次启动时会再次尝试还原: %w", what, err)
	}
	return fmt.Errorf("%s失败，现有数据保持不变，这次恢复已取消: %w", what, err)
}

// restoreDirs 是启动时恢复用到的目录与文件。
type restoreDirs struct {
	dataDir, configDir string
}

func currentRestoreDirs() (restoreDirs, error) {
	dataDir, err := apputils.GetDataDir()
	if err != nil {
		return restoreDirs{}, err
	}
	configDir, err := apputils.GetConfigDir()
	if err != nil {
		return restoreDirs{}, err
	}
	return restoreDirs{dataDir: dataDir, configDir: configDir}, nil
}

func (d restoreDirs) staging() string  { return filepath.Join(d.dataDir, restoreStagingDirName) }
func (d restoreDirs) rollback() string { return filepath.Join(d.dataDir, restoreRollbackDirName) }
func (d restoreDirs) journal() string  { return filepath.Join(d.dataDir, restoreJournalName) }

// prepareStaging 清掉上次留下的暂存目录并重新创建。
func (d restoreDirs) prepareStaging() (string, error) {
	staging := d.staging()
	if err := os.RemoveAll(staging); err != nil {
		return "", fmt.Errorf("清理暂存目录失败: %w", err)
	}
	if err := os.MkdirAll(staging, 0755); err != nil {
		return "", fmt.Errorf("创建暂存目录失败: %w", err)
	}
	return staging, nil
}

// cleanup 删除暂存、回滚目录与暂存、替换前的设置文件，只在没有恢复日志时调用。
func (d restoreDirs) cleanup() {
	_ = os.RemoveAll(d.staging())
	_ = os.RemoveAll(d.rollback())
	_ = os.Remove(filepath.Join(d.configDir, restoreStagedConfigName))
	_ = os.Remove(filepath.Join(d.configDir, restoreConfigBackupName))
}

// dataItem 是数据目录下 name（文件或目录）的替换项，staged 为空时只移走现有的。
func (d restoreDirs) dataItem(name, staged string) restoreSwapItem {
	return restoreSwapItem{Live: filepath.Join(d.dataDir, name), Staged: staged, Backup: filepath.Join(d.rollback(), name)}
}

// databaseItems 是数据库文件与它的 .wal 的替换项：旧的 .wal 不属于新数据库，总是移走。
func (d restoreDirs) databaseItems(stagedDB string) []restoreSwapItem {
	stagedWAL := ""
	if _, err := os.Stat(stagedDB + ".wal"); err == nil {
		stagedWAL = stagedDB + ".wal"
	}
	return []restoreSwapItem{d.dataItem("lunabox.db", stagedDB), d.dataItem("lunabox.db.wal", stagedWAL)}
}

// configItem 把备份里的设置文件复制到配置目录下暂存（与 appconf.json 同目录，替换是重命名），返回它的替换项。
func (d restoreDirs) configItem(backupConfig string) (restoreSwapItem, error) {
	staged := filepath.Join(d.configDir, restoreStagedConfigName)
	if err := apputils.CopyFile(backupConfig, staged); err != nil {
		return restoreSwapItem{}, fmt.Errorf("暂存配置文件失败: %w", err)
	}
	return restoreSwapItem{
		Live:   filepath.Join(d.configDir, "appconf.json"),
		Staged: staged,
		Backup: filepath.Join(d.configDir, restoreConfigBackupName),
	}, nil
}

// stageRestoreDatabase 在暂存目录里准备新的数据库文件：importDir 不为空时把其中的导出导入到新库，否则直接用 rawDB。
func stageRestoreDatabase(stagingDir, importDir, rawDB string) (string, error) {
	if importDir == "" {
		return rawDB, nil
	}
	dbPath := filepath.Join(stagingDir, "lunabox.db")
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return "", fmt.Errorf("打开数据库失败: %w", err)
	}
	err = importDuckDBDatabase(db, importDir)
	if closeErr := db.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return "", fmt.Errorf("导入数据库失败: %w", err)
	}
	return dbPath, nil
}

// commitRestoreSwap 依次替换 items，成功后删除被换下来的旧数据。某一项失败时把已做的替换移回去并返回错误；
// 移不回去时返回 *restoreRollbackError，恢复日志与回滚目录留给下次启动的 RecoverInterruptedRestore。
func commitRestoreSwap(journalPath string, items []restoreSwapItem) error {
	for i := range items {
		_, err := os.Lstat(items[i].Live)
		items[i].Existed = err == nil
		// 回滚目录里不能有上次留下的同名项，否则回滚时会把它当成被换下来的现有数据。
		if err := os.RemoveAll(items[i].Backup); err != nil {
			return fmt.Errorf("清理回滚目录失败: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(items[i].Backup), 0755); err != nil {
			return fmt.Errorf("创建回滚目录失败: %w", err)
		}
	}
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	if err := writeFileSynced(journalPath, data); err != nil {
		return fmt.Errorf("写入恢复日志失败: %w", err)
	}

	fail := func(err error) error {
		if rbErr := rollbackRestoreSwap(items); rbErr != nil {
			return &restoreRollbackError{err: err, rollbackErr: rbErr}
		}
		// 数据已经移回去了；日志删不掉也没关系，下次启动按日志回滚是空操作。
		_ = os.Remove(journalPath)
		return err
	}
	for _, item := range items {
		if item.Existed {
			if err := os.Rename(item.Live, item.Backup); err != nil {
				return fail(fmt.Errorf("移走 %s 失败（文件可能正被其他程序占用）: %w", filepath.Base(item.Live), err))
			}
		}
		if item.Staged != "" {
			if err := os.Rename(item.Staged, item.Live); err != nil {
				return fail(fmt.Errorf("替换 %s 失败: %w", filepath.Base(item.Live), err))
			}
		}
	}
	// 删掉日志才算替换完成：日志还在时下次启动会回滚，所以删不掉就当作失败，现在就回滚。
	if err := os.Remove(journalPath); err != nil {
		return fail(fmt.Errorf("删除恢复日志失败: %w", err))
	}
	for _, item := range items {
		_ = os.RemoveAll(item.Backup)
	}
	return nil
}

// rollbackRestoreSwap 按相反顺序撤销替换，可以重复执行：被换下来的还在回滚目录里就移回原位置；
// 原来不存在的就删掉换上去的。
func rollbackRestoreSwap(items []restoreSwapItem) error {
	var errs []error
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if _, err := os.Lstat(item.Backup); err == nil {
			if err := os.RemoveAll(item.Live); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := os.Rename(item.Backup, item.Live); err != nil {
				errs = append(errs, err)
			}
		} else if !item.Existed {
			if err := os.RemoveAll(item.Live); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// RecoverInterruptedRestore 在执行恢复与打开数据库之前调用：上次的恢复替换到一半就退出了（留下了恢复日志）时，
// 把现有数据移回原样并报告 true；之后清掉暂存与回滚目录。
func RecoverInterruptedRestore() (bool, error) {
	dirs, err := currentRestoreDirs()
	if err != nil {
		return false, err
	}
	return recoverInterruptedRestore(dirs)
}

func recoverInterruptedRestore(dirs restoreDirs) (bool, error) {
	data, err := os.ReadFile(dirs.journal())
	if os.IsNotExist(err) {
		dirs.cleanup()
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("读取恢复日志失败: %w", err)
	}
	var items []restoreSwapItem
	if err := json.Unmarshal(data, &items); err != nil {
		return false, fmt.Errorf("解析恢复日志 %s 失败: %w", dirs.journal(), err)
	}
	if err := rollbackRestoreSwap(items); err != nil {
		return false, fmt.Errorf("还原上次中断的恢复失败（被换下的数据在 %s）: %w", dirs.rollback(), err)
	}
	if err := os.Remove(dirs.journal()); err != nil {
		return false, fmt.Errorf("删除恢复日志失败: %w", err)
	}
	dirs.cleanup()
	return true, nil
}

// writeFileSynced 写入文件并刷到磁盘，恢复日志必须在开始替换前落盘。
func writeFileSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
