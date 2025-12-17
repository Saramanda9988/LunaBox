package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/enums"
	"lunabox/internal/vo"
	"net/http"
	"strings"
	"time"
)

type AiService struct {
	ctx       context.Context
	db        *sql.DB
	appConfig *appconf.AppConfig
}

func NewAiService() *AiService {
	return &AiService{}
}

func (s *AiService) Init(ctx context.Context, db *sql.DB, appConfig *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.appConfig = appConfig
}

// AISummarize 生成AI锐评总结
func (s *AiService) AISummarize(req vo.AISummaryRequest) (vo.AISummaryResponse, error) {
	if s.appConfig.AIAPIKey == "" {
		return vo.AISummaryResponse{}, fmt.Errorf("请先在设置中配置AI API Key")
	}

	// 获取统计数据
	statsData, err := s.getStatsForAI(enums.Period(req.Dimension))
	if err != nil {
		return vo.AISummaryResponse{}, fmt.Errorf("获取统计数据失败: %w", err)
	}

	// 构建prompt
	prompt := s.buildPrompt(statsData)

	// 调用AI API
	summary, err := s.callAIAPI(prompt)
	if err != nil {
		return vo.AISummaryResponse{}, fmt.Errorf("AI调用失败: %w", err)
	}

	return vo.AISummaryResponse{
		Summary:   summary,
		Dimension: req.Dimension,
	}, nil
}

// AIStatsData AI总结所需的统计数据
type AIStatsData struct {
	Dimension         string
	TotalPlayCount    int
	TotalPlayDuration int
	TopGames          []GamePlayInfo
	Timeline          []TimelineInfo
}

type GamePlayInfo struct {
	Name     string
	Duration int
}

type TimelineInfo struct {
	Label    string
	Duration int
}

func (s *AiService) getStatsForAI(dimension enums.Period) (*AIStatsData, error) {
	data := &AIStatsData{
		Dimension: string(dimension),
	}

	var startDateExpr string
	switch dimension {
	case "week":
		startDateExpr = "current_date - INTERVAL 6 DAY"
	case "month":
		startDateExpr = "current_date - INTERVAL 29 DAY"
	case "year":
		startDateExpr = "date_trunc('month', current_date) - INTERVAL 11 MONTH"
	default:
		startDateExpr = "current_date - INTERVAL 6 DAY"
	}

	// 获取总游玩次数和时长
	queryTotal := fmt.Sprintf("SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(duration), 0) FROM play_sessions WHERE start_time >= %s", startDateExpr)
	err := s.db.QueryRowContext(s.ctx, queryTotal).Scan(&data.TotalPlayCount, &data.TotalPlayDuration)
	if err != nil {
		return nil, err
	}

	// 获取Top游戏
	queryLeaderboard := fmt.Sprintf(`
		SELECT g.name, SUM(ps.duration) as total 
		FROM play_sessions ps 
		JOIN games g ON ps.game_id = g.id 
		WHERE ps.start_time >= %s
		GROUP BY g.name 
		ORDER BY total DESC 
		LIMIT 5
	`, startDateExpr)

	rows, err := s.db.QueryContext(s.ctx, queryLeaderboard)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var info GamePlayInfo
		if err := rows.Scan(&info.Name, &info.Duration); err != nil {
			return nil, err
		}
		data.TopGames = append(data.TopGames, info)
	}

	return data, nil
}

func (s *AiService) buildPrompt(data *AIStatsData) string {
	var sb strings.Builder

	periodName := "本周"
	switch data.Dimension {
	case "month":
		periodName = "本月"
	case "year":
		periodName = "今年"
	}

	sb.WriteString(fmt.Sprintf("请根据以下%s游戏统计数据，用幽默风趣的语气写一段锐评总结（100-200字）：\n\n", periodName))
	sb.WriteString(fmt.Sprintf("时间范围：%s\n", periodName))
	sb.WriteString(fmt.Sprintf("总游玩次数：%d 次\n", data.TotalPlayCount))
	sb.WriteString(fmt.Sprintf("总游玩时长：%.1f 小时\n\n", float64(data.TotalPlayDuration)/3600))

	if len(data.TopGames) > 0 {
		sb.WriteString("游玩排行榜：\n")
		for i, game := range data.TopGames {
			sb.WriteString(fmt.Sprintf("%d. %s - %.1f小时\n", i+1, game.Name, float64(game.Duration)/3600))
		}
	}

	sb.WriteString("\n请用轻松幽默的方式点评这位玩家的游戏习惯，可以适当调侃但不要太过分。")

	return sb.String()
}

// OpenAI兼容的API请求/响应结构
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type APIError struct {
	Message string `json:"message"`
}

func (s *AiService) callAIAPI(prompt string) (string, error) {
	baseURL := s.appConfig.AIBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	model := s.appConfig.AIModel
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	reqBody := ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: "你是一个幽默风趣的游戏评论员，擅长用轻松的语气点评玩家的游戏习惯。"},
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(s.ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.appConfig.AIAPIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API请求失败: %s", string(body))
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("API错误: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("AI未返回有效响应")
	}

	return result.Choices[0].Message.Content, nil
}
