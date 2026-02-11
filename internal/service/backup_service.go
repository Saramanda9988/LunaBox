package service

import (
	"context"
	"database/sql"
	"fmt"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
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

// SetupCloudBackup 设置云备份密码（只能设置一次）
func (s *BackupService) SetupCloudBackup(password string) (string, error) {
	// 检查是否已经设置过密码
	if s.config.BackupPassword != "" {
		applog.LogWarningf(s.ctx, "SetupCloudBackup: backup password already set")
		return "", fmt.Errorf("备份密码已设置，无法修改")
	}

	if password == "" {
		applog.LogWarningf(s.ctx, "SetupCloudBackup: backup password is empty")
		return "", fmt.Errorf("备份密码不能为空")
	}

	// 生成用户ID
	userID := utils.GenerateUserID(password)

	// 更新配置
	s.config.BackupUserID = userID
	s.config.BackupPassword = password

	// 立即保存配置到文件
	if err := appconf.SaveConfig(s.config); err != nil {
		applog.LogErrorf(s.ctx, "SetupCloudBackup: failed to save config: %v", err)
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	applog.LogInfof(s.ctx, "SetupCloudBackup: backup password set successfully, user_id: %s", userID)
	return userID, nil
}

// TestS3Connection 测试 S3 连接
func (s *BackupService) TestS3Connection(config appconf.AppConfig) error {
	if err := cloudprovider.TestConnection(s.ctx, cloudprovider.ProviderS3, &config); err != nil {
		applog.LogErrorf(s.ctx, "TestS3Connection: connection test failed: %v", err)
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
		applog.LogErrorf(s.ctx, "StartOneDriveAuth: failed to get auth code: %v", err)
		return "", err
	}
	tokenResp, err := onedrive.ExchangeOneDriveCodeForToken(s.ctx, s.config.OneDriveClientID, code)
	if err != nil {
		applog.LogErrorf(s.ctx, "StartOneDriveAuth: failed to exchange code for token: %v", err)
		return "", err
	}
	return tokenResp.RefreshToken, nil
}

// ExchangeOneDriveCode 用授权码换取 OneDrive token
func (s *BackupService) ExchangeOneDriveCode(code string) (string, error) {
	tokenResp, err := onedrive.ExchangeOneDriveCodeForToken(s.ctx, s.config.OneDriveClientID, code)
	if err != nil {
		applog.LogErrorf(s.ctx, "ExchangeOneDriveCode: failed to exchange code for token: %v", err)
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

// GetFullBackupDir 获取全量数据备份目录
func (s *BackupService) GetFullBackupDir() (string, error) {
	return utils.GetSubDir(filepath.Join("backups", "full"))
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

// GetGameBackups 获取游戏的备份历史（直接读取文件夹，不使用数据库）
func (s *BackupService) GetGameBackups(gameID string) ([]models.GameBackup, error) {
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return nil, err
	}

	gameBackupDir := filepath.Join(backupDir, gameID)
	entries, err := os.ReadDir(gameBackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.GameBackup{}, nil
		}
		return nil, err
	}

	var backups []models.GameBackup
	for _, entry := range entries {
		// 只处理 .zip 文件，跳过目录（如 pre_restore、cloud_download）
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, models.GameBackup{
			Path:      filepath.Join(gameBackupDir, entry.Name()),
			Name:      entry.Name(),
			GameID:    gameID,
			Size:      info.Size(),
			CreatedAt: info.ModTime(), // 使用文件修改时间
		})
	}

	// 按创建时间降序排序
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

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
		return nil, fmt.Errorf("存档路径未设置")
	}
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("存档路径不存在: %s", savePath)
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

	size, err := utils.ZipFileOrDirectory(savePath, backupPath)
	if err != nil {
		return nil, fmt.Errorf("备份失败: %w", err)
	}

	// 获取文件信息以得到准确的修改时间
	stat, err := os.Stat(backupPath)
	if err != nil {
		return nil, err
	}

	backup := &models.GameBackup{
		Path:      backupPath,
		Name:      backupFileName,
		GameID:    gameID,
		Size:      size,
		CreatedAt: stat.ModTime(),
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
		s.DeleteBackup(backups[i].Path)
	}
}

// RestoreBackup 恢复备份到指定时间点（参数改为备份路径）
func (s *BackupService) RestoreBackup(backupPath string) error {
	// 从路径中提取 gameID（路径格式: backups/{gameID}/{timestamp}.zip）
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return err
	}

	// 验证备份路径在合法目录下
	if !strings.HasPrefix(backupPath, backupDir) {
		return fmt.Errorf("无效的备份路径")
	}

	// 从路径提取 gameID
	relPath, _ := filepath.Rel(backupDir, backupPath)
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) < 2 {
		return fmt.Errorf("无效的备份路径格式")
	}
	gameID := parts[0]

	// 检查备份文件是否存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	var savePath string
	err = s.db.QueryRowContext(s.ctx, "SELECT COALESCE(save_path, '') FROM games WHERE id = ?", gameID).Scan(&savePath)
	if err != nil || savePath == "" {
		return fmt.Errorf("存档路径未设置")
	}

	// 先备份当前存档（恢复前备份）
	if _, err := os.Stat(savePath); err == nil {
		preRestoreDir := filepath.Join(backupDir, gameID, "pre_restore")
		os.MkdirAll(preRestoreDir, 0755)
		preRestorePath := filepath.Join(preRestoreDir, fmt.Sprintf("%s_before_restore.zip", time.Now().Format("2006-01-02T15-04-05")))
		_, err := utils.ZipFileOrDirectory(savePath, preRestorePath)
		if err != nil {
			return err
		}
	}

	// 检查原始存档路径是文件还是目录
	// 根据备份前的路径类型来决定恢复方式
	parentDir := filepath.Dir(savePath)
	if err := os.RemoveAll(savePath); err != nil {
		return fmt.Errorf("删除原存档失败: %w", err)
	}

	// 临时解压目录
	tempDir := filepath.Join(backupDir, gameID, "temp_restore")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压到临时目录
	if err := utils.UnzipFile(backupPath, tempDir); err != nil {
		return fmt.Errorf("解压备份失败: %w", err)
	}

	// 检查解压后的内容
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("读取临时目录失败: %w", err)
	}

	// 如果只有一个文件且不是目录，说明备份的是单个文件
	if len(entries) == 1 && !entries[0].IsDir() {
		// 恢复单个文件
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}
		srcFile := filepath.Join(tempDir, entries[0].Name())
		if err := utils.CopyFile(srcFile, savePath); err != nil {
			return fmt.Errorf("恢复文件失败: %w", err)
		}
	} else {
		// 恢复整个目录
		if err := os.MkdirAll(savePath, 0755); err != nil {
			return fmt.Errorf("创建存档目录失败: %w", err)
		}
		if err := utils.CopyDir(tempDir, savePath); err != nil {
			return fmt.Errorf("恢复目录失败: %w", err)
		}
	}

	return nil
}

// DeleteBackup 删除备份（参数改为备份路径）
func (s *BackupService) DeleteBackup(backupPath string) error {
	backupDir, err := s.GetBackupDir()
	if err != nil {
		return err
	}

	// 验证备份路径在合法目录下
	if !strings.HasPrefix(backupPath, backupDir) {
		return fmt.Errorf("无效的备份路径")
	}

	return os.Remove(backupPath)
}

// ========== 游戏存档云备份方法 ==========

// UploadGameBackupToCloud 上传游戏存档到云端（参数改为 backupPath）
func (s *BackupService) UploadGameBackupToCloud(gameID string, backupPath string) error {
	if s.config.BackupUserID == "" {
		return fmt.Errorf("备份用户 ID 未设置")
	}

	provider, err := s.getCloudProvider()
	if err != nil {
		return err
	}

	// 验证备份文件存在
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
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
		return fmt.Errorf("存档路径未设置")
	}

	// 先备份当前存档
	if _, err := os.Stat(savePath); err == nil {
		backupDir, _ := s.GetBackupDir()
		preRestoreDir := filepath.Join(backupDir, gameID, "pre_restore")
		os.MkdirAll(preRestoreDir, 0755)
		preRestorePath := filepath.Join(preRestoreDir, fmt.Sprintf("%s_before_cloud_restore.zip", time.Now().Format("2006-01-02T15-04-05")))
		_, err := utils.ZipFileOrDirectory(savePath, preRestorePath)
		if err != nil {
			return err
		}
	}

	// 获取备份目录用于临时解压
	backupDir, _ := s.GetBackupDir()
	parentDir := filepath.Dir(savePath)
	if err := os.RemoveAll(savePath); err != nil {
		return fmt.Errorf("删除原存档失败: %w", err)
	}

	// 临时解压目录
	tempDir := filepath.Join(backupDir, gameID, "temp_restore")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压到临时目录
	if err := utils.UnzipFile(localPath, tempDir); err != nil {
		return fmt.Errorf("解压备份失败: %w", err)
	}

	// 检查解压后的内容
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("读取临时目录失败: %w", err)
	}

	// 如果只有一个文件且不是目录，说明备份的是单个文件
	if len(entries) == 1 && !entries[0].IsDir() {
		// 恢复单个文件
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %w", err)
		}
		srcFile := filepath.Join(tempDir, entries[0].Name())
		if err := utils.CopyFile(srcFile, savePath); err != nil {
			return fmt.Errorf("恢复文件失败: %w", err)
		}
	} else {
		// 恢复整个目录
		if err := os.MkdirAll(savePath, 0755); err != nil {
			return fmt.Errorf("创建存档目录失败: %w", err)
		}
		if err := utils.CopyDir(tempDir, savePath); err != nil {
			return fmt.Errorf("恢复目录失败: %w", err)
		}
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

// CreateDBBackup 创建数据库备份（包含 covers 文件夹）
func (s *BackupService) CreateDBBackup() (*vo.DBBackupInfo, error) {
	backupDir, err := s.GetDBBackupDir()
	if err != nil {
		return nil, err
	}

	dataDir, err := utils.GetDataDir()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	// 创建临时打包目录，包含数据库导出和 covers
	packDir := filepath.Join(backupDir, fmt.Sprintf("pack_%s", timestamp))
	dbExportDir := filepath.Join(packDir, "database")
	coversDestDir := filepath.Join(packDir, "covers")

	if err := os.MkdirAll(dbExportDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 导出数据库
	exportPath := strings.ReplaceAll(dbExportDir, "\\", "/")
	_, err = s.db.ExecContext(s.ctx, fmt.Sprintf("EXPORT DATABASE '%s'", exportPath))
	if err != nil {
		os.RemoveAll(packDir)
		return nil, fmt.Errorf("导出数据库失败: %w", err)
	}

	// 复制 covers 文件夹（如果存在）
	coversSourceDir := filepath.Join(dataDir, "covers")
	if _, err := os.Stat(coversSourceDir); err == nil {
		if err := utils.CopyDir(coversSourceDir, coversDestDir); err != nil {
			applog.LogWarningf(s.ctx, "CreateDBBackup: failed to copy covers: %v", err)
			// 封面复制失败不影响整体备份，继续执行
		}
	}

	// 打包整个目录
	backupFileName := fmt.Sprintf("lunabox_%s.zip", timestamp)
	backupPath := filepath.Join(backupDir, backupFileName)

	_, err = utils.ZipDirectory(packDir, backupPath)
	os.RemoveAll(packDir)
	if err != nil {
		return nil, fmt.Errorf("压缩备份失败: %w", err)
	}

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
			CreatedAt: info.ModTime(), // 使用本地时间
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

// ========== 全量数据本地备份方法 ==========

// CreateFullDataBackup 创建全量数据备份（数据库 + 应用设置 + 数据目录）
func (s *BackupService) CreateFullDataBackup() (*vo.DBBackupInfo, error) {
	backupDir, err := s.GetFullBackupDir()
	if err != nil {
		return nil, err
	}

	dataDir, err := utils.GetDataDir()
	if err != nil {
		return nil, err
	}
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	packDir := filepath.Join(backupDir, fmt.Sprintf("pack_%s", timestamp))
	dbExportDir := filepath.Join(packDir, "database")

	if err := os.MkdirAll(dbExportDir, 0755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(packDir)

	// 先用 DuckDB EXPORT 获取一致性的数据库快照。
	exportPath := strings.ReplaceAll(dbExportDir, "\\", "/")
	_, err = s.db.ExecContext(s.ctx, fmt.Sprintf("EXPORT DATABASE '%s'", exportPath))
	if err != nil {
		return nil, fmt.Errorf("导出数据库失败: %w", err)
	}

	// 复制配置文件
	configPath := filepath.Join(configDir, "appconf.json")
	if _, err := os.Stat(configPath); err == nil {
		if err := utils.CopyFile(configPath, filepath.Join(packDir, "appconf.json")); err != nil {
			return nil, fmt.Errorf("复制配置文件失败: %w", err)
		}
	}

	// 复制关键数据目录
	for _, dirName := range []string{"covers", "backgrounds", "logs"} {
		srcDir := filepath.Join(dataDir, dirName)
		if _, err := os.Stat(srcDir); err != nil {
			continue
		}
		if err := utils.CopyDir(srcDir, filepath.Join(packDir, dirName)); err != nil {
			return nil, fmt.Errorf("复制目录 %s 失败: %w", dirName, err)
		}
	}

	// 复制 backups（排除 full，避免把正在创建的全量备份自身递归打包）
	backupsSourceDir := filepath.Join(dataDir, "backups")
	if _, err := os.Stat(backupsSourceDir); err == nil {
		backupsDestDir := filepath.Join(packDir, "backups")
		if err := os.MkdirAll(backupsDestDir, 0755); err != nil {
			return nil, fmt.Errorf("创建备份目录失败: %w", err)
		}
		entries, err := os.ReadDir(backupsSourceDir)
		if err != nil {
			return nil, fmt.Errorf("读取备份目录失败: %w", err)
		}
		for _, entry := range entries {
			if entry.Name() == "full" {
				continue
			}
			srcPath := filepath.Join(backupsSourceDir, entry.Name())
			dstPath := filepath.Join(backupsDestDir, entry.Name())
			if entry.IsDir() {
				if err := utils.CopyDir(srcPath, dstPath); err != nil {
					return nil, fmt.Errorf("复制备份子目录失败: %w", err)
				}
			} else {
				if err := utils.CopyFile(srcPath, dstPath); err != nil {
					return nil, fmt.Errorf("复制备份文件失败: %w", err)
				}
			}
		}
	}

	backupFileName := fmt.Sprintf("lunabox_full_%s.zip", timestamp)
	backupPath := filepath.Join(backupDir, backupFileName)

	_, err = utils.ZipDirectory(packDir, backupPath)
	if err != nil {
		return nil, fmt.Errorf("压缩全量备份失败: %w", err)
	}

	stat, err := os.Stat(backupPath)
	if err != nil {
		return nil, err
	}

	s.config.LastFullBackupTime = time.Now().Format(time.RFC3339)
	//retention := s.config.LocalDBBackupRetention
	//if retention <= 0 {
	//	retention = 5
	//}
	//s.cleanupOldFullBackups(retention)

	return &vo.DBBackupInfo{
		Path:      backupPath,
		Name:      backupFileName,
		Size:      stat.Size(),
		CreatedAt: stat.ModTime(),
	}, nil
}

// GetFullDataBackups 获取全量数据备份列表
func (s *BackupService) GetFullDataBackups() (*vo.DBBackupStatus, error) {
	backupDir, err := s.GetFullBackupDir()
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
		LastBackupTime: s.config.LastFullBackupTime,
		Backups:        backups,
	}, nil
}

// ScheduleFullDataRestore 安排全量数据恢复（下次启动时执行）
func (s *BackupService) ScheduleFullDataRestore(backupPath string) error {
	backupDir, err := s.GetFullBackupDir()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(backupPath, backupDir) {
		return fmt.Errorf("无效的备份路径")
	}
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	// 全量恢复包含数据库，清理数据库待恢复任务，避免重复恢复。
	s.config.PendingDBRestore = ""
	s.config.PendingFullRestore = backupPath
	return nil
}

// DeleteFullDataBackup 删除全量数据备份
func (s *BackupService) DeleteFullDataBackup(backupPath string) error {
	backupDir, err := s.GetFullBackupDir()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(backupPath, backupDir) {
		return fmt.Errorf("无效的备份路径")
	}
	return os.Remove(backupPath)
}

// cleanupOldFullBackups 清理旧的全量数据备份
func (s *BackupService) cleanupOldFullBackups(retention int) {
	status, err := s.GetFullDataBackups()
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
// TODO: 前端提供此功能的按钮
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

	// 只有在启用云备份、配置完整且开启数据库自动上传时才上传
	if s.config.CloudBackupEnabled && s.config.BackupUserID != "" && s.config.AutoUploadDBToCloud {
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

// ========== 全量数据恢复（启动时调用）==========

// ExecuteFullDataRestore 执行全量数据恢复（在 OnStartup 中、打开数据库前调用）
func ExecuteFullDataRestore(config *appconf.AppConfig) (bool, error) {
	if config.PendingFullRestore == "" {
		return false, nil
	}

	backupPath := config.PendingFullRestore
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		config.PendingFullRestore = ""
		appconf.SaveConfig(config)
		return false, fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	dataDir, err := utils.GetDataDir()
	if err != nil {
		return false, err
	}
	configDir, err := utils.GetConfigDir()
	if err != nil {
		return false, err
	}

	tempDir, err := os.MkdirTemp("", "lunabox_full_restore_*")
	if err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := utils.UnzipForRestore(backupPath, tempDir); err != nil {
		return false, fmt.Errorf("解压全量备份失败: %w", err)
	}

	// 先恢复数据库
	dbPath := filepath.Join(dataDir, "lunabox.db")
	dbImportDir := filepath.Join(tempDir, "database")
	rawDBPath := filepath.Join(tempDir, "lunabox.db")

	os.Remove(dbPath)
	os.Remove(dbPath + ".wal")

	if _, err := os.Stat(dbImportDir); err == nil {
		db, err := sql.Open("duckdb", dbPath)
		if err != nil {
			return false, fmt.Errorf("打开数据库失败: %w", err)
		}

		importPath := strings.ReplaceAll(dbImportDir, "\\", "/")
		_, err = db.Exec(fmt.Sprintf("IMPORT DATABASE '%s'", importPath))
		db.Close()
		if err != nil {
			return false, fmt.Errorf("导入数据库失败: %w", err)
		}
	} else if _, err := os.Stat(rawDBPath); err == nil {
		if err := utils.CopyFile(rawDBPath, dbPath); err != nil {
			return false, fmt.Errorf("恢复数据库文件失败: %w", err)
		}
	} else {
		return false, fmt.Errorf("全量备份中缺少数据库内容")
	}

	// 恢复应用数据目录
	for _, dirName := range []string{"covers", "backgrounds", "logs", "backups"} {
		srcDir := filepath.Join(tempDir, dirName)
		if _, err := os.Stat(srcDir); err != nil {
			continue
		}

		dstDir := filepath.Join(dataDir, dirName)
		if err := os.RemoveAll(dstDir); err != nil {
			return false, fmt.Errorf("清理目录 %s 失败: %w", dirName, err)
		}
		if err := utils.CopyDir(srcDir, dstDir); err != nil {
			return false, fmt.Errorf("恢复目录 %s 失败: %w", dirName, err)
		}
	}

	// 恢复配置文件
	backupConfigPath := filepath.Join(tempDir, "appconf.json")
	if _, err := os.Stat(backupConfigPath); err == nil {
		configPath := filepath.Join(configDir, "appconf.json")
		if err := utils.CopyFile(backupConfigPath, configPath); err != nil {
			return false, fmt.Errorf("恢复配置文件失败: %w", err)
		}
	}

	// 重新加载配置并清理待恢复标记，避免重复执行
	restoredConfig, err := appconf.LoadConfig()
	if err != nil {
		restoredConfig = config
	}
	restoredConfig.PendingFullRestore = ""
	restoredConfig.PendingDBRestore = ""
	if err := appconf.SaveConfig(restoredConfig); err != nil {
		return false, fmt.Errorf("保存恢复后配置失败: %w", err)
	}
	*config = *restoredConfig

	return true, nil
}

// ========== 数据库恢复（启动时调用）==========

// ExecuteDBRestore 执行数据库恢复（在 OnStartup 中、打开数据库前调用）
// 支持新格式（包含 database/ 和 covers/ 子目录）和旧格式（直接是数据库导出文件）
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

	dataDir, err := utils.GetDataDir()
	if err != nil {
		return false, err
	}
	dbPath := filepath.Join(dataDir, "lunabox.db")

	tempDir := filepath.Join(dataDir, "backups", "database", "restore_temp")
	os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return false, fmt.Errorf("创建临时目录失败: %w", err)
	}

	if err := utils.UnzipForRestore(backupPath, tempDir); err != nil {
		os.RemoveAll(tempDir)
		return false, fmt.Errorf("解压备份失败: %w", err)
	}

	// 检测备份格式：新格式有 database/ 子目录，旧格式直接是数据库文件
	dbImportDir := tempDir
	coversBackupDir := ""
	if _, err := os.Stat(filepath.Join(tempDir, "database")); err == nil {
		// 新格式
		dbImportDir = filepath.Join(tempDir, "database")
		coversBackupDir = filepath.Join(tempDir, "covers")
	}

	os.Remove(dbPath)
	os.Remove(dbPath + ".wal")

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return false, fmt.Errorf("打开数据库失败: %w", err)
	}

	importPath := strings.ReplaceAll(dbImportDir, "\\", "/")
	_, err = db.Exec(fmt.Sprintf("IMPORT DATABASE '%s'", importPath))
	db.Close()

	if err != nil {
		os.RemoveAll(tempDir)
		return false, fmt.Errorf("导入数据库失败: %w", err)
	}

	// 恢复 covers 文件夹（如果备份中包含）
	if coversBackupDir != "" {
		if _, err := os.Stat(coversBackupDir); err == nil {
			coversDestDir := filepath.Join(dataDir, "covers")
			// 先清空现有 covers 目录
			os.RemoveAll(coversDestDir)
			if err := utils.CopyDir(coversBackupDir, coversDestDir); err != nil {
				// 封面恢复失败不影响整体恢复，只记录警告
				fmt.Printf("警告: 恢复封面图片失败: %v\n", err)
			}
		}
	}

	os.RemoveAll(tempDir)

	config.PendingDBRestore = ""
	appconf.SaveConfig(config)
	return true, nil
}
