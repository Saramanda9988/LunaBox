package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/models"
	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/service/cloudprovider/onedrive"
	"lunabox/internal/utils"
	"lunabox/internal/vo"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type BackupService struct {
	ctx    context.Context
	db     *sql.DB
	config *appconf.AppConfig
}

func NewBackupService() *BackupService {
	return &BackupService{}
}

func (s *BackupService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.config = config
}

// getCloudProvider 获取云备份提供商
func (s *BackupService) getCloudProvider() (cloudprovider.CloudStorageProvider, error) {
	return cloudprovider.NewCloudProvider(s.ctx, s.config)
}

// ========== 云备份配置相关方法 ==========

// SetupCloudBackup 设置云备份（生成 user-id）
func (s *BackupService) SetupCloudBackup(password string) (string, error) {
	if password == "" {
		runtime.LogWarningf(s.ctx, "SetupCloudBackup: backup password is empty")
		return "", fmt.Errorf("备份密码不能为空")
	}
	userID := utils.GenerateUserID(password)
	s.config.BackupUserID = userID
	s.config.BackupPassword = password
	return userID, nil
}

// TestS3Connection 测试 S3 连接
func (s *BackupService) TestS3Connection(config appconf.AppConfig) error {
	if err := cloudprovider.TestConnection(s.ctx, cloudprovider.ProviderS3, &config); err != nil {
		runtime.LogErrorf(s.ctx, "TestS3Connection: connection test failed: %v", err)
		return fmt.Errorf("连接测试失败: %w", err)
	}
	return nil
}

// TestOneDriveConnection 测试 OneDrive 连接
func (s *BackupService) TestOneDriveConnection(config appconf.AppConfig) error {
	return cloudprovider.TestConnection(s.ctx, cloudprovider.ProviderOneDrive, &config)
}

// GetOneDriveAuthURL 获取 OneDrive 授权 URL
func (s *BackupService) GetOneDriveAuthURL() string {
	return onedrive.GetOneDriveAuthURL(s.config.OneDriveClientID)
}

// StartOneDriveAuth 启动 OneDrive 授权流程（使用本地回调服务器）
func (s *BackupService) StartOneDriveAuth() (string, error) {
	code, err := onedrive.StartOneDriveAuthServer(s.ctx, 5*time.Minute)
	if err != nil {
		runtime.LogErrorf(s.ctx, "StartOneDriveAuth: failed to get auth code: %v", err)
		return "", err
	}
	tokenResp, err := onedrive.ExchangeOneDriveCodeForToken(s.ctx, s.config.OneDriveClientID, code)
	if err != nil {
		runtime.LogErrorf(s.ctx, "StartOneDriveAuth: failed to exchange code for token: %v", err)
		return "", err
	}
	return tokenResp.RefreshToken, nil
}

// ExchangeOneDriveCode 用授权码换取 OneDrive token
func (s *BackupService) ExchangeOneDriveCode(code string) (string, error) {
	tokenResp, err := onedrive.ExchangeOneDriveCodeForToken(s.ctx, s.config.OneDriveClientID, code)
	if err != nil {
		runtime.LogErrorf(s.ctx, "ExchangeOneDriveCode: failed to exchange code for token: %v", err)
		return "", err
	}
	return tokenResp.RefreshToken, nil
}

// GetCloudBackupStatus 获取云备份状态
func (s *BackupService) GetCloudBackupStatus() vo.CloudBackupStatus {
	return vo.CloudBackupStatus{
		Enabled:    s.config.CloudBackupEnabled,
		Configured: cloudprovider.IsConfigured(s.config),
		UserID:     s.config.BackupUserID,
		Provider:   s.config.CloudBackupProvider,
	}
}

// ========== 本地备份目录相关方法 ==========

// GetBackupDir 获取备份根目录
func (s *BackupService) GetBackupDir() (string, error) {
	return utils.GetSubDir("backups")
}

// GetDBBackupDir 获取数据库备份目录
func (s *BackupService) GetDBBackupDir() (string, error) {
	return utils.GetSubDir(filepath.Join("backups", "database"))
}

// OpenBackupFolder 打开备份文件夹
func (s *BackupService) OpenBackupFolder(gameID string) error {
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return err
	}
	gameBackupDir := filepath.Join(backupDir, gameID)
	return utils.OpenDirectory(gameBackupDir)
}

// ========== 游戏存档本地备份方法 ==========

// GetGameBackups 获取游戏的备份历史
func (s *BackupService) GetGameBackups(gameID string) ([]models.GameBackup, error) {
	query := `SELECT id, game_id, backup_path, size, created_at 
		FROM game_backups WHERE game_id = ? ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(s.ctx, query, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query backups: %w", err)
	}
	defer rows.Close()

	var backups []models.GameBackup
	for rows.Next() {
		var backup models.GameBackup
		err := rows.Scan(&backup.ID, &backup.GameID, &backup.BackupPath, &backup.Size, &backup.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backup: %w", err)
		}
		backups = append(backups, backup)
	}
	return backups, nil
}

// CreateBackup 创建游戏存档备份
func (s *BackupService) CreateBackup(gameID string) (*models.GameBackup, error) {
	var savePath string
	err := s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}
	if savePath == "" {
		return nil, fmt.Errorf("存档目录未设置")
	}
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("存档目录不存在: %s", savePath)
	}

	backupDir, err := s.GetBackupDir()
	if err != nil {
		return nil, err
	}
	gameBackupDir := filepath.Join(backupDir, gameID)
	if err := os.MkdirAll(gameBackupDir, 0755); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupFileName := fmt.Sprintf("%s.zip", timestamp)
	backupPath := filepath.Join(gameBackupDir, backupFileName)

	size, err := utils.ZipDirectory(savePath, backupPath)
	if err != nil {
		return nil, fmt.Errorf("备份失败: %w", err)
	}

	backup := &models.GameBackup{
		ID:         uuid.New().String(),
		GameID:     gameID,
		BackupPath: backupPath,
		Size:       size,
		CreatedAt:  time.Now(),
	}

	_, err = s.db.ExecContext(s.ctx,
		"INSERT INTO game_backups (id, game_id, backup_path, size, created_at) VALUES (?, ?, ?, ?, ?)",
		backup.ID, backup.GameID, backup.BackupPath, backup.Size, backup.CreatedAt)
	if err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("failed to save backup record: %w", err)
	}

	s.cleanupOldLocalBackups(gameID)

	return backup, nil
}

// cleanupOldLocalBackups 清理旧的本地游戏备份
func (s *BackupService) cleanupOldLocalBackups(gameID string) {
	retention := s.config.LocalBackupRetention
	if retention <= 0 {
		retention = 20
	}

	backups, err := s.GetGameBackups(gameID)
	if err != nil || len(backups) <= retention {
		return
	}

	for i := retention; i < len(backups); i++ {
		s.DeleteBackup(backups[i].ID)
	}
}

// RestoreBackup 恢复备份到指定时间点
func (s *BackupService) RestoreBackup(backupID string) error {
	var backup models.GameBackup
	err := s.db.QueryRowContext(s.ctx,
		"SELECT id, game_id, backup_path, size, created_at FROM game_backups WHERE id = ?", backupID).
		Scan(&backup.ID, &backup.GameID, &backup.BackupPath, &backup.Size, &backup.CreatedAt)
	if err != nil {
		return fmt.Errorf("备份记录不存在")
	}

	var savePath string
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", backup.GameID).Scan(&savePath)
	if err != nil || savePath == "" {
		return fmt.Errorf("存档目录未设置")
	}
	if _, err := os.Stat(backup.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backup.BackupPath)
	}

	// 先备份当前存档（恢复前备份）
	if _, err := os.Stat(savePath); err == nil {
		backupDir, _ := s.GetBackupDir()
		preRestoreDir := filepath.Join(backupDir, backup.GameID, "pre_restore")
		os.MkdirAll(preRestoreDir, 0755)
		preRestorePath := filepath.Join(preRestoreDir, fmt.Sprintf("%s_before_restore.zip", time.Now().Format("2006-01-02T15-04-05")))
		_, err := utils.ZipDirectory(savePath, preRestorePath)
		if err != nil {
			return err
		}
	}

	if err := os.RemoveAll(savePath); err != nil {
		return fmt.Errorf("清空存档目录失败: %w", err)
	}
	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("创建存档目录失败: %w", err)
	}
	if err := utils.UnzipFile(backup.BackupPath, savePath); err != nil {
		return fmt.Errorf("恢复失败: %w", err)
	}
	return nil
}

// DeleteBackup 删除备份
func (s *BackupService) DeleteBackup(backupID string) error {
	var backupPath string
	err := s.db.QueryRowContext(s.ctx, "SELECT backup_path FROM game_backups WHERE id = ?", backupID).Scan(&backupPath)
	if err != nil {
		return fmt.Errorf("备份记录不存在")
	}
	os.Remove(backupPath)
	_, err = s.db.ExecContext(s.ctx, "DELETE FROM game_backups WHERE id = ?", backupID)
	return err
}

// ========== 游戏存档云备份方法 ==========

// UploadGameBackupToCloud 上传游戏存档到云端
func (s *BackupService) UploadGameBackupToCloud(gameID string, backupID string) error {
	if s.config.BackupUserID == "" {
		return fmt.Errorf("备份用户 ID 未设置")
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return err
	}

	var backupPath string
	err = s.db.QueryRowContext(s.ctx, "SELECT backup_path FROM game_backups WHERE id = ?", backupID).Scan(&backupPath)
	if err != nil {
		return fmt.Errorf("备份记录不存在")
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	cloudPath := provider.GetCloudPath(s.config.BackupUserID, fmt.Sprintf("saves/%s/%s.zip", gameID, timestamp))

	// 确保文件夹存在 (OneDrive 需要)
	folderPath := provider.GetCloudPath(s.config.BackupUserID, fmt.Sprintf("saves/%s", gameID))
	provider.EnsureDir(s.ctx, folderPath)

	if err := provider.UploadFile(s.ctx, cloudPath, backupPath); err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}

	// 更新 latest
	latestPath := provider.GetCloudPath(s.config.BackupUserID, fmt.Sprintf("saves/%s/latest.zip", gameID))
	provider.UploadFile(s.ctx, latestPath, backupPath)

	s.cleanupOldCloudBackups(gameID)
	return nil
}

// GetCloudGameBackups 获取云端游戏备份列表
func (s *BackupService) GetCloudGameBackups(gameID string) ([]vo.CloudBackupItem, error) {
	if s.config.BackupUserID == "" {
		return nil, fmt.Errorf("备份用户 ID 未设置")
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return nil, err
	}

	listPath := provider.GetCloudPath(s.config.BackupUserID, fmt.Sprintf("saves/%s/", gameID))
	keys, err := provider.ListObjects(s.ctx, listPath)
	if err != nil {
		return nil, err
	}

	return s.parseCloudBackupItems(keys, ""), nil
}

// DownloadCloudBackup 从云端下载备份
func (s *BackupService) DownloadCloudBackup(cloudKey string, gameID string) (string, error) {
	provider, err := s.getCloudProvider()
	if err != nil {
		return "", err
	}

	backupDir, err := s.GetBackupDir()
	if err != nil {
		return "", err
	}
	cloudDownloadDir := filepath.Join(backupDir, gameID, "cloud_download")
	os.MkdirAll(cloudDownloadDir, 0755)

	destPath := filepath.Join(cloudDownloadDir, filepath.Base(cloudKey))
	if err := provider.DownloadFile(s.ctx, cloudKey, destPath); err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	return destPath, nil
}

// RestoreFromCloud 从云端恢复备份
func (s *BackupService) RestoreFromCloud(cloudKey string, gameID string) error {
	localPath, err := s.DownloadCloudBackup(cloudKey, gameID)
	if err != nil {
		return err
	}

	var savePath string
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil || savePath == "" {
		return fmt.Errorf("存档目录未设置")
	}

	// 先备份当前存档
	if _, err := os.Stat(savePath); err == nil {
		backupDir, _ := s.GetBackupDir()
		preRestoreDir := filepath.Join(backupDir, gameID, "pre_restore")
		os.MkdirAll(preRestoreDir, 0755)
		preRestorePath := filepath.Join(preRestoreDir, fmt.Sprintf("%s_before_cloud_restore.zip", time.Now().Format("2006-01-02T15-04-05")))
		_, err := utils.ZipDirectory(savePath, preRestorePath)
		if err != nil {
			return err
		}
	}

	if err := os.RemoveAll(savePath); err != nil {
		return fmt.Errorf("清空存档目录失败: %w", err)
	}
	if err := os.MkdirAll(savePath, 0755); err != nil {
		return fmt.Errorf("创建存档目录失败: %w", err)
	}
	if err := utils.UnzipFile(localPath, savePath); err != nil {
		return fmt.Errorf("恢复失败: %w", err)
	}
	return nil
}

// cleanupOldCloudBackups 清理旧的云端备份
func (s *BackupService) cleanupOldCloudBackups(gameID string) {
	retention := s.config.CloudBackupRetention
	if retention <= 0 {
		retention = 20
	}

	items, err := s.GetCloudGameBackups(gameID)
	if err != nil || len(items) <= retention {
		return
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return
	}

	for i := retention; i < len(items); i++ {
		provider.DeleteObject(s.ctx, items[i].Key)
	}
}

// ========== 数据库本地备份方法 ==========

// CreateDBBackup 创建数据库备份
func (s *BackupService) CreateDBBackup() (*vo.DBBackupInfo, error) {
	backupDir, err := s.GetDBBackupDir()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	exportDir := filepath.Join(backupDir, fmt.Sprintf("export_%s", timestamp))
	backupFileName := fmt.Sprintf("lunabox_%s.zip", timestamp)
	backupPath := filepath.Join(backupDir, backupFileName)

	exportPath := strings.ReplaceAll(exportDir, "\\", "/")
	_, err = s.db.ExecContext(s.ctx, fmt.Sprintf("EXPORT DATABASE '%s'", exportPath))
	if err != nil {
		return nil, fmt.Errorf("导出数据库失败: %w", err)
	}

	_, err = utils.ZipDirectory(exportDir, backupPath)
	if err != nil {
		os.RemoveAll(exportDir)
		return nil, fmt.Errorf("压缩备份失败: %w", err)
	}
	os.RemoveAll(exportDir)

	stat, err := os.Stat(backupPath)
	if err != nil {
		return nil, err
	}

	s.config.LastDBBackupTime = time.Now().Format(time.RFC3339)
	retention := s.config.LocalDBBackupRetention
	if retention <= 0 {
		retention = 10
	}
	s.cleanupOldDBBackups(retention)

	return &vo.DBBackupInfo{
		Path:      backupPath,
		Name:      backupFileName,
		Size:      stat.Size(),
		CreatedAt: time.Now(),
	}, nil
}

// GetDBBackups 获取数据库备份列表
func (s *BackupService) GetDBBackups() (*vo.DBBackupStatus, error) {
	backupDir, err := s.GetDBBackupDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}

	var backups []vo.DBBackupInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, vo.DBBackupInfo{
			Path:      filepath.Join(backupDir, entry.Name()),
			Name:      entry.Name(),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return &vo.DBBackupStatus{
		LastBackupTime: s.config.LastDBBackupTime,
		Backups:        backups,
	}, nil
}

// ScheduleDBRestore 安排数据库恢复（下次启动时执行）
func (s *BackupService) ScheduleDBRestore(backupPath string) error {
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}
	s.config.PendingDBRestore = backupPath
	return nil
}

// DeleteDBBackup 删除数据库备份
func (s *BackupService) DeleteDBBackup(backupPath string) error {
	backupDir, err := s.GetDBBackupDir()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(backupPath, backupDir) {
		return fmt.Errorf("无效的备份路径")
	}
	return os.Remove(backupPath)
}

// cleanupOldDBBackups 清理旧的数据库备份
func (s *BackupService) cleanupOldDBBackups(retention int) {
	status, err := s.GetDBBackups()
	if err != nil || len(status.Backups) <= retention {
		return
	}
	for i := retention; i < len(status.Backups); i++ {
		os.Remove(status.Backups[i].Path)
	}
}

// ========== 数据库云备份方法 ==========

// UploadDBBackupToCloud 上传数据库备份到云端
func (s *BackupService) UploadDBBackupToCloud(backupPath string) error {
	if s.config.BackupUserID == "" {
		return fmt.Errorf("备份用户 ID 未设置")
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return err
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	fileName := filepath.Base(backupPath)
	cloudKey := provider.GetCloudPath(s.config.BackupUserID, fmt.Sprintf("database/%s", fileName))

	folderPath := provider.GetCloudPath(s.config.BackupUserID, "database")
	provider.EnsureDir(s.ctx, folderPath)

	if err := provider.UploadFile(s.ctx, cloudKey, backupPath); err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}

	latestKey := provider.GetCloudPath(s.config.BackupUserID, "database/latest.zip")
	provider.UploadFile(s.ctx, latestKey, backupPath)

	s.cleanupOldCloudDBBackups()
	return nil
}

// GetCloudDBBackups 获取云端数据库备份列表
func (s *BackupService) GetCloudDBBackups() ([]vo.CloudBackupItem, error) {
	if s.config.BackupUserID == "" {
		return nil, fmt.Errorf("备份用户 ID 未设置")
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return nil, err
	}

	prefix := provider.GetCloudPath(s.config.BackupUserID, "database/")
	keys, err := provider.ListObjects(s.ctx, prefix)
	if err != nil {
		return nil, err
	}

	return s.parseCloudBackupItems(keys, "lunabox_"), nil
}

// DownloadCloudDBBackup 从云端下载数据库备份
func (s *BackupService) DownloadCloudDBBackup(cloudKey string) (string, error) {
	provider, err := s.getCloudProvider()
	if err != nil {
		return "", err
	}

	backupDir, err := s.GetDBBackupDir()
	if err != nil {
		return "", err
	}
	cloudDownloadDir := filepath.Join(backupDir, "cloud_download")
	os.MkdirAll(cloudDownloadDir, 0755)

	destPath := filepath.Join(cloudDownloadDir, filepath.Base(cloudKey))
	if err := provider.DownloadFile(s.ctx, cloudKey, destPath); err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	return destPath, nil
}

// ScheduleDBRestoreFromCloud 从云端下载并安排数据库恢复
func (s *BackupService) ScheduleDBRestoreFromCloud(cloudKey string) error {
	localPath, err := s.DownloadCloudDBBackup(cloudKey)
	if err != nil {
		return err
	}
	return s.ScheduleDBRestore(localPath)
}

// cleanupOldCloudDBBackups 清理旧的云端数据库备份
func (s *BackupService) cleanupOldCloudDBBackups() {
	retention := s.config.CloudBackupRetention
	if retention <= 0 {
		retention = 10
	}

	items, err := s.GetCloudDBBackups()
	if err != nil || len(items) <= retention {
		return
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return
	}

	for i := retention; i < len(items); i++ {
		provider.DeleteObject(s.ctx, items[i].Key)
	}
}

// CreateAndUploadDBBackup 创建数据库备份并上传到云端
func (s *BackupService) CreateAndUploadDBBackup() (*vo.DBBackupInfo, error) {
	backup, err := s.CreateDBBackup()
	if err != nil {
		return nil, err
	}

	if s.config.CloudBackupEnabled && s.config.BackupUserID != "" {
		if err := s.UploadDBBackupToCloud(backup.Path); err != nil {
			return backup, fmt.Errorf("本地备份成功，但云端上传失败: %w", err)
		}
	}
	return backup, nil
}

// ========== 辅助方法 ==========

// parseCloudBackupItems 解析云端备份列表
func (s *BackupService) parseCloudBackupItems(keys []string, prefix string) []vo.CloudBackupItem {
	var items []vo.CloudBackupItem
	for _, key := range keys {
		if strings.HasSuffix(key, "latest.zip") {
			continue
		}
		name := filepath.Base(key)
		displayName := name
		name = strings.TrimPrefix(name, prefix)
		name = strings.TrimSuffix(name, ".zip")
		t, _ := time.Parse("2006-01-02T15-04-05", name)

		items = append(items, vo.CloudBackupItem{
			Key:       key,
			Name:      displayName,
			CreatedAt: t,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

// ========== 数据库恢复（启动时调用）==========

// ExecuteDBRestore 执行数据库恢复（在 OnStartup 中、打开数据库前调用）
func ExecuteDBRestore(config *appconf.AppConfig) (bool, error) {
	if config.PendingDBRestore == "" {
		return false, nil
	}

	backupPath := config.PendingDBRestore

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		config.PendingDBRestore = ""
		appconf.SaveConfig(config)
		return false, fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	execDir, err := utils.GetDataDir()
	if err != nil {
		return false, err
	}
	dbPath := filepath.Join(execDir, "lunabox.db")

	tempDir := filepath.Join(execDir, "backups", "database", "restore_temp")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}

	if err := utils.UnzipForRestore(backupPath, tempDir); err != nil {
		os.RemoveAll(tempDir)
		return false, fmt.Errorf("解压备份失败: %w", err)
	}

	os.Remove(dbPath)
	os.Remove(dbPath + ".wal")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return false, fmt.Errorf("打开数据库失败: %w", err)
	}

	importPath := strings.ReplaceAll(tempDir, "\\", "/")
	_, err = db.Exec(fmt.Sprintf("IMPORT DATABASE '%s'", importPath))
	db.Close()
	os.RemoveAll(tempDir)

	if err != nil {
		return false, fmt.Errorf("导入数据库失败: %w", err)
	}

	config.PendingDBRestore = ""
	appconf.SaveConfig(config)
	return true, nil
}
