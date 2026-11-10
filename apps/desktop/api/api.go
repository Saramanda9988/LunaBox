// Package api 是界面调用的后端接口，对应原版前端调用的 Wails 绑定（frontend/bindings/lunabox/backend/service）。
// 界面只依赖这里的接口，由 host 用 packages/backend 的真实服务实现（host.Backend.API）。
// 方法与 packages/backend/service 同名同签名；界面用到新的服务方法时在这里补上。
package api

import (
	"context"
	"time"

	"lunabox/backend/appconf"
	"lunabox/backend/common/enums"
	"lunabox/backend/common/vo"
	"lunabox/backend/models"
	"lunabox/backend/service"
	launcherpkg "lunabox/backend/service/launcher"
	"lunabox/backend/utils/processutils"
)

// Services 是界面用到的全部后端能力，各页面通过 env.Env.Svc 使用。
type Services struct {
	Config  *appconf.AppConfig // 与服务共用的当前配置
	DataDir string             // 数据目录（封面、背景等本地文件相对它）
	Events  Events             // 服务发出的事件
	// SystemTimeZone 是系统的 IANA 时区（如 Asia/Shanghai），原版取 Intl 的 timeZone；只在配置没有 time_zone 时读取。
	// 不知道时为空。
	SystemTimeZone string

	AppConfig   ConfigService
	Home        HomeService
	Game        GameService
	Tag         TagService
	Stats       StatsService
	Template    TemplateService
	AI          AIService
	Session     SessionService
	Category    CategoryService
	Backup      BackupService
	Start       StartService
	Integration IntegrationService
	Progress    GameProgressService
	Review      GameReviewService
	Bangumi     AccountService[vo.BangumiAuthStatus, vo.BangumiProfile]
	Hikarinagi  AccountService[vo.HikarinagiAuthStatus, vo.HikarinagiProfile]
	Kungal      AccountService[vo.KungalAuthStatus, vo.KungalProfile]
	Download    DownloadService
	Version     VersionService
	Update      UpdateService
	Portable    PortableSetupService
	CloudSync   CloudSyncService
	Presets     GameFilterPresetService
	Import      ImportService
}

// Events 是服务事件的订阅（原版的 Events.On）：fn 总在窗口的主线程上调用，data 是事件的第一个参数。
type Events interface {
	On(name string, fn func(data any))
}

// ConfigService 对应 service.ConfigService。
type ConfigService interface {
	GetAppConfig() (appconf.AppConfig, error)
	UpdateAppConfig(newConfig appconf.AppConfig) error
	SafeQuit()
	SelectDirectory(title string) (string, error)
	SelectAndCropBackgroundImage() (string, error)
	SaveCroppedBackgroundImage(srcPath string, x, y, width, height int) (string, error)
	PreviewGameLibraryPathChange(newPath string) (service.GameLibraryPathChangePreview, error)
	ApplyGameLibraryPathChange(newPath string, syncPaths bool) (service.GameLibraryPathChangeResult, error)
	ExportLogsZip() (string, error)
	OpenDataDirectory() (string, error)
}

// HomeService 对应 service.HomeService。
type HomeService interface {
	GetHomePageData() (vo.HomePageData, error)
}

// GameService 对应 service.GameService。
type GameService interface {
	GetGameByID(id string) (models.Game, error)
	UpdateGame(game models.Game) error
	DeleteGame(id string) error
	DeleteGames(ids []string) error
	BatchUpdateStatus(ids []string, status string) error
	FindGamesWithMissingPath() (vo.MissingPathScanResult, error)
	UpdateGameProcessName(gameID string, processName string) error
	GetRunningProcesses() ([]processutils.ProcessInfo, error)

	SelectGameExecutable(currentPath string) (string, error)
	SelectWineRunnerExecutable(currentPath string) (string, error)
	SelectGameDirectory(currentPath string) (string, error)
	SelectSaveDirectory(rootPath string) (string, error)
	SelectSaveFile(rootPath string) (string, error)
	SelectCoverImage(gameID string) (string, error)
	SaveCoverImageDataURL(gameID string, dataURL string) (string, error)
	DownloadCoverImage(gameID string, coverURL string) (string, error)
	ExportCoverImage(gameID string) (string, error)
	ExportLaunchShortcut(gameID string) (string, error)
	OpenLocalPath(path string) error
	FindGameGuideDocuments(gameID string) ([]vo.GameGuideDocument, error)
	OpenGameGuideDocument(gameID string, relativePath string) error

	// 按名称或 ID 获取元数据、从远程更新游戏都会请求网络，界面用 ctx 取消（*Context 版本）。
	FetchMetadataByNameContext(ctx context.Context, name string) ([]vo.GameMetadataFromWebVO, error)
	FetchMetadataFromWebContext(ctx context.Context, req vo.MetadataRequest) (vo.GameMetadataFromWebVO, error)
	AddGameFromWebMetadata(meta vo.GameMetadataFromWebVO) error
	SelectCoverImageWithTempID() (string, error)
	SaveCoverImageDataURLWithTempID(dataURL string) (string, error)
	UpdateGameFromRemoteBySourceContext(ctx context.Context, gameID string, source enums.SourceType) error
	UpdateGameFromRemoteWithFieldsContext(ctx context.Context, gameID string, fields []enums.MetadataUpdateField) error
	ResolveMetadataSourceIDs(game models.Game) map[enums.SourceType]string
	RefreshGamesMetadataWithFields(gameIDs []string, fields []enums.MetadataUpdateField) (vo.MetadataRefreshResult, error)
	RefreshAllGamesMetadataWithFields(fields []enums.MetadataUpdateField) (vo.MetadataRefreshResult, error)
	SetDefaultMetadataSource(gameID string, source enums.SourceType) error
	UpsertGameMetadataSource(gameID string, source enums.SourceType, sourceID string) error
	DeleteGameMetadataSource(gameID string, source enums.SourceType) error
	PreviewLegacyGameMetadataSourceIDs() (service.GameIDEnrichmentPreview, error)
	EnrichLegacyGameMetadataSourceIDs() (service.GameIDEnrichmentResult, error)
	StartRemoteCoverImageDownloadTask() (string, error)
}

// TagService 对应 service.TagService。
type TagService interface {
	GetTagsByGame(gameID string) ([]models.GameTag, error)
	AddUserTag(gameID string, tagName string) error
	DeleteTag(tagID string) error
}

// StatsService 对应 service.StatsService。
type StatsService interface {
	GetGameStats(req vo.GameStatsRequest) (vo.GameDetailStats, error)
	GetGlobalPeriodStats(req vo.PeriodStatsRequest) (vo.PeriodStats, error)
	GetPosterGames(startDate, endDate string) ([]vo.PosterGame, error)
}

// TemplateService 对应 service.TemplateService（统计页的美化导出）。
type TemplateService interface {
	ListTemplates() ([]vo.TemplateInfo, error)
	PrepareExportData(stats vo.PeriodStats, aiSummary string) (vo.StatsExportData, error)
	RenderTemplate(req vo.RenderTemplateRequest) (vo.RenderTemplateResponse, error)
	OpenTemplatesDir() error
	ExportRenderedHTML(base64Data string) error
}

// AIService 对应 service.AiService。
type AIService interface {
	AISummarize(req vo.AISummaryRequest) (vo.AISummaryResponse, error)
}

// SessionService 对应 service.SessionService。
type SessionService interface {
	GetPlaySessions(gameID string) ([]models.PlaySession, error)
	AddPlaySession(gameID string, startTime time.Time, durationSeconds int) (models.PlaySession, error)
	DeletePlaySession(sessionID string) error
}

// CategoryService 对应 service.CategoryService。
type CategoryService interface {
	GetCategories() ([]vo.CategoryVO, error)
	GetCategoryByID(id string) (vo.CategoryVO, error)
	GetCategoriesByGame(gameID string) ([]vo.CategoryVO, error)
	GetCategoryGames(req vo.CategoryGameListRequest) (vo.GameListResponse, error)
	SearchCategoryGameCandidates(req vo.CategoryGameCandidateRequest) (vo.GameListResponse, error)
	AddCategory(name string, emoji string) error
	UpdateCategory(id, name, emoji string) error
	DeleteCategory(id string) error
	DeleteCategories(ids []string) error
	AddGameToCategory(gameID, categoryID string) error
	AddGamesToCategories(gameIDs []string, categoryIDs []string) error
	RemoveGameFromCategory(gameID, categoryID string) error
	RemoveGamesFromCategory(gameIDs []string, categoryID string) error
}

// BackupService 对应 service.BackupService：游戏存档备份、数据库与完整数据备份、云备份账号。
type BackupService interface {
	GetGameBackups(gameID string) ([]models.GameBackup, error)
	CreateBackup(gameID string) (*models.GameBackup, error)
	RestoreBackup(backupPath string) error
	DeleteBackup(backupPath string) error
	OpenBackupFolder(gameID string) error
	GetCloudBackupStatus() vo.CloudBackupStatus
	GetCloudGameBackups(gameID string) ([]vo.CloudBackupItem, error)
	UploadGameBackupToCloud(gameID string, backupPath string) error
	RestoreFromCloud(cloudKey string, gameID string) error

	GetDBBackups() (*vo.DBBackupStatus, error)
	GetCloudDBBackups() ([]vo.CloudBackupItem, error)
	CreateAndUploadDBBackup() (*vo.DBBackupInfo, error)
	CreateAndUploadDBBackupForQuit() (*vo.DBBackupInfo, error)
	UploadDBBackupToCloud(backupPath string) error
	DeleteDBBackup(backupPath string) error
	ScheduleDBRestore(backupPath string) error
	ScheduleDBRestoreFromCloud(cloudKey string) error

	SelectBackupSavePath() (string, error)
	SelectBackupRestorePath() (string, error)
	CreateFullDataBackup(savePath string) error
	ScheduleFullDataRestore(backupPath string) error

	SetupCloudBackup(password string) (string, error)
	TestS3Connection(config appconf.AppConfig) error
	TestOneDriveConnection(config appconf.AppConfig) error
	TestWebDAVConnection(config appconf.AppConfig) error
	TestUmbraConnection(config appconf.AppConfig) error
	StartOneDriveAuthContext(ctx context.Context, clientID string) (string, error)
	StartUmbraAuth(config appconf.AppConfig) error
	CancelUmbraAuth() bool
	GetUmbraUserProfile(config appconf.AppConfig) (*vo.UmbraUserProfile, error)
	LogoutUmbra(config appconf.AppConfig) error
}

// StartService 对应 service.StartService。
type StartService interface {
	StartGameWithTracking(gameID string) (bool, error)
	StartGameWithOptions(gameID string, options launcherpkg.LaunchOptions) (bool, error)
	EndCurrentPlaySession(gameID string) error
}

// IntegrationService 对应 service.IntegrationService（Steam 集成与 Wine / Proton 兼容层工具）。
type IntegrationService interface {
	GetGameSteamStatus(gameID string) (service.SteamLaunchStatus, error)
	BatchImportGamesToSteam(gameIDs []string) (service.SteamBatchImportResult, error)
	ImportGameToSteamWithOptions(gameID string, launchOptions string, compatibilityTool string) (service.SteamImportResult, error)
	ApplyGameSteamSettings(gameID string, compatibilityTool *string, launchOptions string) error
	GetGameSteamCompatibility(gameID string) (service.SteamCompatibilityInfo, error)
	GetLocalProtonTools() []service.LocalProtonTool
	RestartSteamClient() error
	OpenGameSteamProtonPrefix(gameID string) (string, error)
	GetGameCompatibilityTools(gameID string) (service.GameCompatibilityToolsInfo, error)
	OpenGameCompatibilityTool(gameID string, action string) (string, error)
}

// GameProgressService 对应 service.GameProgressService。
type GameProgressService interface {
	GetGameProgress(gameID string) (*models.GameProgress, error)
	ListGameProgresses(gameID string) ([]models.GameProgress, error)
	UpsertGameProgress(gp models.GameProgress) (*models.GameProgress, error)
}

// GameReviewService 对应 service.GameReviewService。
type GameReviewService interface {
	GetGameReview(gameID string) (*models.GameReview, error)
	SaveGameReview(review models.GameReview) (*models.GameReview, error)
	SyncGameReview(gameID string, providers []enums.SourceType) (vo.GameReviewSyncResult, error)
}

// AccountService 是 Bangumi、Hikarinagi、Kungal 三个账号来源共有的授权与同步方法，Status、Profile 是各自的 vo 类型。
// StartAuthContext 等待浏览器里的授权回调，ctx 取消时停止等待。
type AccountService[Status, Profile any] interface {
	GetAuthStatus() (Status, error)
	GetProfile() (Profile, error)
	StartAuthContext(ctx context.Context) (Status, error)
	Disconnect() (Status, error)
	SyncAllGameStatuses() (vo.RemoteStatusSyncProgress, error)
}

// DownloadService 对应 service.DownloadService。
type DownloadService interface {
	GetPendingInstall() *vo.InstallRequest
	StartDownload(req vo.InstallRequest) (string, error)
	GetDownloadTasks() []service.DownloadTask
	CheckDownloadImportStates(requests []vo.DownloadImportStateRequest) ([]vo.DownloadImportState, error)
	PauseDownload(taskID string) error
	ResumeDownload(taskID string) error
	RetryDownload(taskID string) error
	CancelDownload(taskID string) error
	DeleteDownloadTask(taskID string) error
	OpenDownloadTaskLocation(taskID string) error
	ImportDownloadTaskAsGame(taskID string) error
}

// VersionService 对应 service.VersionService。
type VersionService interface {
	GetVersionInfo() map[string]string
	GetGOOS() string
	SupportsBackgroundProcessMute() bool
}

// UpdateService 对应 service.UpdateService。
type UpdateService interface {
	CheckForUpdates() (*service.UpdateCheckResult, error)
	CheckForUpdatesOnStartup() (*service.UpdateCheckResult, error)
	SkipVersion(ver string) error
	DownloadAndApplyUpdate(manifestURL string) (*service.UpdateApplyResult, error)
}

// PortableSetupService 对应 service.PortableSetupService。
type PortableSetupService interface {
	GetStatus() (service.PortableSetupStatus, error)
	RegisterProtocol() (service.PortableSetupStatus, error)
	UnregisterProtocol() (service.PortableSetupStatus, error)
	RegisterCLIPath() (service.PortableSetupStatus, error)
	UnregisterCLIPath() (service.PortableSetupStatus, error)
}

// CloudSyncService 对应 service.CloudSyncService。
type CloudSyncService interface {
	GetCloudSyncStatus() vo.CloudSyncStatus
	SyncNow() (vo.CloudSyncStatus, error)
}

// GameFilterPresetService 对应 service.GameFilterPresetService（列表查询在 env.Backend）。
type GameFilterPresetService interface {
	CreateGameFilterPreset(req vo.SaveGameFilterPresetRequest) (models.GameFilterPreset, error)
	DeleteGameFilterPreset(id string) error
}

// ImportService 对应 service.ImportService：从其他启动器导入、批量扫描本地目录与拖放导入。
type ImportService interface {
	SelectZipFile() (string, error)
	SelectJSONFile() (string, error)
	SelectYukiHubBackup() (string, error)
	SelectVniteDirectory() (string, error)
	SelectReinaManagerDatabase() (string, error)
	PreviewImport(zipPath string) ([]service.PreviewGame, error)
	PreviewPlayniteImport(jsonPath string) ([]service.PreviewGame, error)
	PreviewYukiHubImport(backupPath string) ([]service.PreviewGame, error)
	PreviewVniteImport(vniteDir string) ([]service.PreviewGame, error)
	PreviewReinaManagerImport(dbPath string) ([]service.PreviewGame, error)
	PreviewSteamLocalImport() ([]service.PreviewGame, error)
	ImportFromPotatoVNWithSelection(zipPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)
	ImportFromPlayniteWithSelection(jsonPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)
	ImportFromYukiHubWithSelection(backupPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)
	ImportFromVniteWithSelection(vniteDir string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)
	ImportFromReinaManagerWithSelection(dbPath string, skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)
	ImportFromSteamLocalWithSelection(skipNoPath bool, samePathAction string, selections []vo.ImportSelection) (service.ImportResult, error)

	SelectLibraryDirectory(initialDirectory string) (service.LibraryDirectorySelection, error)
	ScanLibraryDirectoryWithOptions(libraryPath string, options vo.BatchImportScanOptions) (vo.BatchImportScanResult, error)
	ProcessDroppedPathsWithOptions(paths []string, options vo.BatchImportScanOptions) (vo.BatchImportScanResult, error)
	FetchMetadataForCandidateWithPreferenceContext(ctx context.Context, searchName string, preferredSource enums.SourceType) (vo.BatchImportMetadataMatchResult, error)
	CheckImportMetadataDuplicates(requests []vo.ImportMetadataDuplicateRequest) ([]vo.ImportMetadataDuplicateResult, error)
	BatchImportGames(candidates []vo.BatchImportCandidate) (service.ImportResult, error)
}
