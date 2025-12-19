package service

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/models"
	"lunabox/internal/models/potatovn"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ImportResult 导入结果
type ImportResult struct {
	Success      int      `json:"success"`       // 成功导入数量
	Skipped      int      `json:"skipped"`       // 跳过数量（已存在）
	Failed       int      `json:"failed"`        // 失败数量
	FailedNames  []string `json:"failed_names"`  // 失败的游戏名称
	SkippedNames []string `json:"skipped_names"` // 跳过的游戏名称
}

type ImportService struct {
	ctx         context.Context
	db          *sql.DB
	config      *appconf.AppConfig
	gameService *GameService
}

func NewImportService() *ImportService {
	return &ImportService{}
}

func (s *ImportService) Init(ctx context.Context, db *sql.DB, config *appconf.AppConfig, gameService *GameService) {
	s.ctx = ctx
	s.db = db
	s.config = config
	s.gameService = gameService
}

// SelectZipFile 选择要导入的 ZIP 文件
func (s *ImportService) SelectZipFile() (string, error) {
	selection, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择 PotatoVN 导出的 ZIP 文件",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "ZIP 文件",
				Pattern:     "*.zip",
			},
		},
	})
	return selection, err
}

// ImportFromPotatoVN 从 PotatoVN 导出的 ZIP 文件导入数据
func (s *ImportService) ImportFromPotatoVN(zipPath string) (ImportResult, error) {
	result := ImportResult{
		FailedNames:  []string{},
		SkippedNames: []string{},
	}

	// 打开 ZIP 文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return result, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_import_*")
	if err != nil {
		return result, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 解压文件
	if err := s.extractZip(zipReader, tempDir); err != nil {
		return result, fmt.Errorf("解压失败: %w", err)
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		return result, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		return result, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		return result, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 导入每个游戏
	for _, galgame := range galgames {
		gameName := galgame.GetDisplayName()

		// 检查是否已存在
		if existingNames[strings.ToLower(gameName)] {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, gameName)
			continue
		}

		// 转换并导入游戏
		game := s.convertToGame(galgame, tempDir)

		if err := s.gameService.AddGame(game); err != nil {
			result.Failed++
			result.FailedNames = append(result.FailedNames, gameName)
			continue
		}

		existingNames[strings.ToLower(gameName)] = true
		result.Success++
	}

	return result, nil
}

// extractZip 解压 ZIP 文件到指定目录
func (s *ImportService) extractZip(zipReader *zip.ReadCloser, destDir string) error {
	for _, file := range zipReader.File {
		filePath := filepath.Join(destDir, file.Name)

		// 防止 ZIP Slip 攻击
		if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法的文件路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		// 解压文件
		destFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			destFile.Close()
			return err
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

// convertToGame 将 PotatoVN 的 Galgame 转换为本地的 Game 模型
func (s *ImportService) convertToGame(galgame potatovn.Galgame, tempDir string) models.Game {
	game := models.Game{
		ID:         uuid.New().String(),
		Name:       galgame.GetDisplayName(),
		Company:    galgame.Developer.Value,
		Summary:    galgame.Description.Value,
		Path:       galgame.GetExePath(),
		SavePath:   galgame.GetSavePath(),
		SourceType: s.mapRssTypeToSourceType(galgame.RssType),
		SourceID:   galgame.GetSourceID(),
		CreatedAt:  galgame.AddTime,
		CachedAt:   time.Now(),
	}

	// 处理封面图片
	if galgame.ImagePath.Value != "" && galgame.ImagePath.Value != potatovn.DefaultImagePath {
		// 尝试从解压目录中获取封面图片
		coverPath := s.resolveCoverPath(galgame.ImagePath.Value, tempDir)
		if coverPath != "" {
			// 将封面图片复制到应用的封面目录
			savedPath, err := s.saveCoverImage(coverPath, game.ID)
			if err == nil {
				game.CoverURL = savedPath
			}
		}
	}

	// 如果 CreatedAt 是零值，使用当前时间
	if game.CreatedAt.IsZero() {
		game.CreatedAt = time.Now()
	}

	return game
}

// mapRssTypeToSourceType 将 PotatoVN 的 RssType 映射到本地的 SourceType
func (s *ImportService) mapRssTypeToSourceType(rssType potatovn.RssType) enums.SourceType {
	switch rssType {
	case potatovn.RssTypeBangumi:
		return enums.Bangumi
	case potatovn.RssTypeVndb:
		return enums.VNDB
	case potatovn.RssTypeYmgal:
		return enums.Ymgal
	default:
		return enums.Local
	}
}

// resolveCoverPath 解析封面图片路径
func (s *ImportService) resolveCoverPath(imagePath string, tempDir string) string {
	// PotatoVN 的图片路径格式通常是 ".\\Images\\xxx_cover" 或相对路径
	// 需要转换为绝对路径

	// 移除开头的 ".\" 或 "./"
	cleanPath := strings.TrimPrefix(imagePath, ".\\")
	cleanPath = strings.TrimPrefix(cleanPath, "./")

	// 替换反斜杠为正斜杠
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")

	// 构建完整路径
	fullPath := filepath.Join(tempDir, cleanPath)

	// 检查文件是否存在，可能需要添加扩展名
	extensions := []string{"", ".png", ".jpg", ".jpeg", ".webp", ".gif"}
	for _, ext := range extensions {
		testPath := fullPath + ext
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
	}

	return ""
}

// saveCoverImage 保存封面图片到应用的封面目录
func (s *ImportService) saveCoverImage(srcPath string, gameID string) (string, error) {
	// 获取应用程序目录
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	appDir := filepath.Dir(execPath)

	// 获取封面保存目录
	coverDir := filepath.Join(appDir, "covers")
	if err := os.MkdirAll(coverDir, os.ModePerm); err != nil {
		return "", err
	}

	// 获取源文件的扩展名
	ext := filepath.Ext(srcPath)
	if ext == "" {
		ext = ".png"
	}

	// 生成目标文件名
	destFileName := fmt.Sprintf("%s%s", gameID, ext)
	destPath := filepath.Join(coverDir, destFileName)

	// 复制文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", err
	}

	// 返回相对路径或可访问的 URL
	return fmt.Sprintf("/local/covers/%s", destFileName), nil
}

// PreviewImport 预览导入内容（不实际导入）
func (s *ImportService) PreviewImport(zipPath string) ([]PreviewGame, error) {
	// 打开 ZIP 文件
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开 ZIP 文件: %w", err)
	}
	defer zipReader.Close()

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "potatovn_preview_*")
	if err != nil {
		return nil, fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 只解压 data.galgames.json
	for _, file := range zipReader.File {
		if file.Name == "data.galgames.json" {
			filePath := filepath.Join(tempDir, file.Name)
			destFile, err := os.Create(filePath)
			if err != nil {
				return nil, err
			}

			srcFile, err := file.Open()
			if err != nil {
				destFile.Close()
				return nil, err
			}

			_, err = io.Copy(destFile, srcFile)
			srcFile.Close()
			destFile.Close()

			if err != nil {
				return nil, err
			}
			break
		}
	}

	// 读取 data.galgames.json
	galgamesPath := filepath.Join(tempDir, "data.galgames.json")
	galgamesData, err := os.ReadFile(galgamesPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取 data.galgames.json: %w", err)
	}

	var galgames []potatovn.Galgame
	if err := json.Unmarshal(galgamesData, &galgames); err != nil {
		return nil, fmt.Errorf("解析 data.galgames.json 失败: %w", err)
	}

	// 获取现有游戏列表，用于去重检查
	existingGames, err := s.gameService.GetGames()
	if err != nil {
		return nil, fmt.Errorf("获取现有游戏列表失败: %w", err)
	}
	existingNames := make(map[string]bool)
	for _, g := range existingGames {
		existingNames[strings.ToLower(g.Name)] = true
	}

	// 构建预览列表
	var previews []PreviewGame
	for _, galgame := range galgames {
		name := galgame.GetDisplayName()
		preview := PreviewGame{
			Name:       name,
			Developer:  galgame.Developer.Value,
			SourceType: string(s.mapRssTypeToSourceType(galgame.RssType)),
			Exists:     existingNames[strings.ToLower(name)],
			AddTime:    galgame.AddTime,
		}
		previews = append(previews, preview)
	}

	return previews, nil
}

// PreviewGame 预览导入的游戏信息
type PreviewGame struct {
	Name       string    `json:"name"`
	Developer  string    `json:"developer"`
	SourceType string    `json:"source_type"`
	Exists     bool      `json:"exists"`
	AddTime    time.Time `json:"add_time"`
}
