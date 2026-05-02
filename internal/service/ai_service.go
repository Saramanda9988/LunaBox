package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"lunabox/internal/appconf"
	"lunabox/internal/applog"
	enums2 "lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
	"lunabox/internal/utils"
	"net/http"
	"strings"
	"time"
)

type AiService struct {
	ctx       context.Context
	db        *sql.DB
	appConfig *appconf.AppConfig
	stats     AIStatsProvider
}

func NewAiService() *AiService {
	return &AiService{}
}

func (s *AiService) Init(ctx context.Context, db *sql.DB, appConfig *appconf.AppConfig) {
	s.ctx = ctx
	s.db = db
	s.appConfig = appConfig

	builder := NewAIStatsBuilder()
	builder.Init(ctx, db, appConfig)
	s.stats = builder
}

// AISummarize 生成AI锐评总结
func (s *AiService) AISummarize(req vo.AISummaryRequest) (vo.AISummaryResponse, error) {
	if s.appConfig.AIAPIKey == "" {
		applog.LogError(s.ctx, "[AIService] please configure AI API Key first")
		return vo.AISummaryResponse{}, fmt.Errorf("please configure AI API Key first")
	}

	// 确定防剧透等级（请求覆盖 > 全局配置 > 默认 none）
	spoilerLevel := req.SpoilerLevel
	if spoilerLevel == "" {
		spoilerLevel = s.appConfig.AISpoilerLevel
	}
	spoilerLevel = NormalizeSpoilerLevel(spoilerLevel)

	// 获取统计数据
	statsData, err := s.getStatsForAI(enums2.Period(req.Dimension))
	if err != nil {
		applog.LogError(s.ctx, "[AIService] fail to get stats: "+err.Error())
		return vo.AISummaryResponse{}, fmt.Errorf("获取统计数据失败: %w", err)
	}

	// 构建三层 Prompt
	systemPrompt := s.buildSystemPrompt(statsData, spoilerLevel)
	contextPrompt := s.buildContextPrompt(statsData)
	taskPrompt := s.buildTaskPrompt(statsData)

	// 构造消息列表
	messages := []vo.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: contextPrompt + "\n\n" + taskPrompt},
	}

	// 调用AI API（含 WebSearch 工具调用）
	webSearchEnabled := s.appConfig.AIWebSearchEnabled
	summary, webSearchUsed, err := s.callAIAPIWithTools(messages, webSearchEnabled)
	if err != nil {
		applog.LogError(s.ctx, "[AIService] fail to call AI: "+err.Error())
		return vo.AISummaryResponse{}, fmt.Errorf("AI调用失败: %w", err)
	}

	return vo.AISummaryResponse{
		Summary:       summary,
		Dimension:     req.Dimension,
		WebSearchUsed: webSearchUsed,
	}, nil
}

// ─────────────────────────────────────────
// DATA LAYER
// ─────────────────────────────────────────

func (s *AiService) getStatsForAI(dimension enums2.Period) (*AIStatsData, error) {
	if s.stats == nil {
		builder := NewAIStatsBuilder()
		builder.Init(s.ctx, s.db, s.appConfig)
		s.stats = builder
	}
	return s.stats.Build(dimension)
}

// ─────────────────────────────────────────
// PROMPT LAYER（三层分离）
// ─────────────────────────────────────────

// buildSystemPrompt Layer 1: 人设 + 防剧透指令 + 输出约束
func (s *AiService) buildSystemPrompt(data *AIStatsData, spoilerLevel string) string {
	var sb strings.Builder

	// 人设
	persona := s.appConfig.AISystemPrompt
	if persona == "" {
		persona = string(enums2.DefaultSystemPrompt)
	}
	sb.WriteString(persona)
	sb.WriteString("\n\n")

	sb.WriteString("[角色执行优先级 - MUST FOLLOW]\n")
	sb.WriteString("- 你首先是上方定义的人设，不是中立助手、新闻编辑或统计播报员。\n")
	sb.WriteString("- 先按人设开口，再使用数据做支撑；所有评论都应像在直接点评玩家本人。\n")
	sb.WriteString("- 后续所有通用要求都服务于人设表达；若与人设冲突，以人设要求优先，但仍必须遵守语言、剧透和篇幅限制。\n")
	sb.WriteString("- 后续所有通用要求都服务于人设表达；若与人设冲突，以人设要求优先，但仍必须遵守语言、剧透和篇幅限制。\n")
	sb.WriteString("- 允许有鲜明态度，但判断必须能从给定数据中找到依据。\n\n")

	// 工具环境提醒
	sb.WriteString("[环境说明]\n")
	sb.WriteString("用户使用的程序是 LunaBox，一款本地游戏管理和启动器软件。请勿在回答中提及该软件名称。\n\n")
	sb.WriteString(fmt.Sprintf("用户的语言是 %s，请严格返回相同语言的回答\n", s.appConfig.Language))
	// 防剧透指令（仅在有游戏时注入，且非 full 等级）
	if spoilerLevel != "full" && len(data.TopGames) > 0 {
		sb.WriteString("[剧透控制 - MUST FOLLOW]\n")
		for _, g := range data.TopGames {
			if g.SpoilerBoundary == "none" || spoilerLevel == "none" {
				// 最严格：禁止任何剧情细节
				if g.Route != "" || g.ProgressNote != "" {
					sb.WriteString(fmt.Sprintf("- 《%s》：严禁提及任何具体剧情、结局、角色关系或路线发展。仅允许讨论游戏类型、风格与操作体验。\n", g.Name))
				} else {
					sb.WriteString(fmt.Sprintf("- 《%s》：严禁提及任何剧情细节、结局和角色命运。\n", g.Name))
				}
			} else if g.SpoilerBoundary == "chapter_end" {
				boundary := g.Route
				if g.Route == "" {
					boundary = g.ProgressNote
				}
				sb.WriteString(fmt.Sprintf("- 《%s》：用户当前进度「%s」。可讨论该章节已发生的内容，严禁剧透后续章节、分支或结局。\n", g.Name, boundary))
			} else if g.SpoilerBoundary == "route_end" {
				sb.WriteString(fmt.Sprintf("- 《%s》：用户正在进行「%s」路线。可讨论该路线完整内容，严禁剧透其他路线或真结局。\n", g.Name, g.Route))
			}
			// full/mild: 不添加限制
		}

		if spoilerLevel == "mild" {
			sb.WriteString("提示：在上述具体约束之外，请尽量避免主动透露关键转折和结局，保持适度谨慎。\n")
		}
		sb.WriteString("\n")
	}

	// 输出约束
	sb.WriteString("[输出约束]\n")
	sb.WriteString("- 字数控制在 200-350 字。\n")
	sb.WriteString("- 请以自然分段输出，不要出现小标题、编号或“玩家画像”“重点作品点评”等字样。不要写成标题列表、统计报告、攻略说明或媒体测评。\n")
	sb.WriteString("- 请以自然分段输出，不要出现小标题、编号或“玩家画像”“重点作品点评”等字样。不要写成标题列表、统计报告、攻略说明或媒体测评。\n")
	sb.WriteString("- 语气、措辞、是否调侃、是否使用 emoji、是否加入括号包含的动作描写等表现形式必须服从上方人设。适当添加，不要为了加而加\n")
	sb.WriteString("- 评论重点是玩家的口味、习惯、状态与游玩时间特征\n")
	sb.WriteString("- 如果引用游戏简介或 WebSearch 信息，只能当作辅助证据，不要连续大段改写原文。必要时提及游戏名是可以的\n")
	sb.WriteString("- 如果数据量少，请聚焦最明显的一两个特征，不要为了凑结构硬写。\n")

	return sb.String()
}

// buildContextPrompt Layer 2: 结构化数据快照（统计 + 作息 + 游戏条目）
func (s *AiService) buildContextPrompt(data *AIStatsData) string {
	var sb strings.Builder

	sb.WriteString("=== 游玩数据快照 ===\n\n")

	// 统计摘要
	sb.WriteString(fmt.Sprintf("本期总览：游玩 %d 次，合计 %.1f 小时（数据范围：%s）\n\n", data.TotalPlayCount, float64(data.TotalPlayDuration)/3600, data.DateRange))

	// 游戏条目
	if len(data.TopGames) > 0 {
		sb.WriteString("游玩排行（Top 5）：\n")
		for i, g := range data.TopGames {
			sb.WriteString(fmt.Sprintf("%d. 《%s》", i+1, g.Name))
			if g.Company != "" {
				sb.WriteString(fmt.Sprintf("（%s）", g.Company))
			}
			sb.WriteString(fmt.Sprintf(" — %.1f 小时", float64(g.Duration)/3600))
			if len(g.Categories) > 0 {
				sb.WriteString(fmt.Sprintf("  [%s]", strings.Join(g.Categories, " / ")))
			}
			if g.Status != "" && g.Status != "not_started" {
				statusLabel := map[string]string{
					"playing":   "游玩中",
					"completed": "已通关",
					"on_hold":   "搁置中",
				}[g.Status]
				if statusLabel != "" {
					sb.WriteString(fmt.Sprintf("  <%s>", statusLabel))
				}
			}
			sb.WriteString("\n")
			if g.Summary != "" {
				sb.WriteString(fmt.Sprintf("   题材参考（勿长篇复述）：%s\n", g.Summary))
			}
			if g.ProgressNote != "" {
				sb.WriteString(fmt.Sprintf("   玩家进度备注：%s\n", g.ProgressNote))
			}
		}
		sb.WriteString("\n")
	}

	// 作息分析（基于近期 session，时间已按配置时区转换）
	if len(data.RecentSessions) >= 3 {
		nightCount, afternoonCount, morningCount, otherCount := 0, 0, 0, 0
		weekdayCount, weekendCount := 0, 0
		for _, sess := range data.RecentSessions {
			switch {
			case sess.Hour >= 22 || sess.Hour < 4:
				nightCount++
			case sess.Hour >= 13 && sess.Hour < 19:
				afternoonCount++
			case sess.Hour >= 8 && sess.Hour < 12:
				morningCount++
			default:
				otherCount++
			}
			if sess.DayOfWeek == 0 || sess.DayOfWeek == 6 {
				weekendCount++
			} else {
				weekdayCount++
			}
		}
		// 时段分布：给出原始数字，让 AI 自行解读
		sb.WriteString(fmt.Sprintf("游玩时段分布（近 %d 条，%s，请自行解读用户作息和游玩时间特点）：\n", len(data.RecentSessions), data.DateRange))
		timeParts := []string{}
		if nightCount > 0 {
			timeParts = append(timeParts, fmt.Sprintf("深夜22-4时 %d 次", nightCount))
		}
		if afternoonCount > 0 {
			timeParts = append(timeParts, fmt.Sprintf("下午13-19时 %d 次", afternoonCount))
		}
		if morningCount > 0 {
			timeParts = append(timeParts, fmt.Sprintf("上午8-12时 %d 次", morningCount))
		}
		if otherCount > 0 {
			timeParts = append(timeParts, fmt.Sprintf("其他时段 %d 次", otherCount))
		}
		if len(timeParts) > 0 {
			sb.WriteString("  " + strings.Join(timeParts, " / ") + "\n")
		}
		// 星期分布：明确标注节假日无法区分
		sb.WriteString(fmt.Sprintf("  工作日(周一至五) %d 次 / 周末(周六日) %d 次（按自然星期统计，节假日无法区分）\n\n",
			weekdayCount, weekendCount))
	}

	return sb.String()
}

// buildTaskPrompt Layer 3: 任务指令
func (s *AiService) buildTaskPrompt(data *AIStatsData) string {
	periodName := "最近7天"
	switch data.Dimension {
	case "week":
		periodName = "最近7天"
	case "month":
		periodName = "最近1个月"
	}

	return fmt.Sprintf(`=== 任务指令 ===

优先抓住最鲜明的一两个特征，自然组织内容，不要自我解释结构，也不要套固定模板。
游戏题材、标签、进度、作息和 WebSearch 信息都只能作为你下判断时的证据

请严格遵守[剧透控制]规则和[输出约束]规则，以你的人设，围绕用户在「%s」里的游玩表现写一段锐评。`, periodName)
}

// ─────────────────────────────────────────
// API CALL LAYER（含 WebSearch 工具调用）
// ─────────────────────────────────────────

var webSearchToolDef = vo.Tool{
	Type: "function",
	Function: vo.ToolFunction{
		Name:        "web_search",
		Description: "Search for game background, genre tags, developer info, or general reception. DO NOT search for plot spoilers.",
		Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Search query"}},"required":["query"]}`),
	},
}

// callAIAPIWithTools 调用 AI API，支持多轮 WebSearch Tool Use
func (s *AiService) callAIAPIWithTools(messages []vo.Message, enableWebSearch bool) (string, bool, error) {
	baseURL := s.appConfig.AIBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := s.appConfig.AIModel
	if model == "" {
		model = "gpt-3.5-turbo"
	}

	apiURL := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
	webSearchUsed := false

	// 首轮（可能含 tools）
	var tools []vo.Tool
	if enableWebSearch {
		tools = []vo.Tool{webSearchToolDef}
	}

	rawResp, err := s.doAPICall(apiURL, model, messages, tools)
	if err != nil {
		return "", false, err
	}

	// 处理 tool_calls（最多 3 轮，防止无限循环）
	for round := 0; round < 3; round++ {
		if len(rawResp.Choices) == 0 {
			break
		}
		choice := rawResp.Choices[0]
		if choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) == 0 {
			break
		}

		// 追加 assistant 消息
		messages = append(messages, choice.Message)

		// 执行所有 tool calls
		for _, tc := range choice.Message.ToolCalls {
			if tc.Function.Name != "web_search" {
				continue
			}
			var args struct {
				Query string `json:"query"`
			}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				continue
			}
			searchResult := s.executeWebSearch(args.Query)
			webSearchUsed = true
			messages = append(messages, vo.Message{
				Role:       "tool",
				Content:    searchResult,
				ToolCallID: tc.ID,
			})
		}

		// 第二轮不再传 tools（防止无限循环）
		rawResp, err = s.doAPICall(apiURL, model, messages, nil)
		if err != nil {
			return "", webSearchUsed, err
		}
	}

	if len(rawResp.Choices) == 0 {
		return "", webSearchUsed, fmt.Errorf("AI未返回有效响应")
	}
	return rawResp.Choices[0].Message.Content, webSearchUsed, nil
}

// doAPICall 向 AI API 发送一次请求
func (s *AiService) doAPICall(apiURL, model string, messages []vo.Message, tools []vo.Tool) (*vo.ChatCompletionResponse, error) {
	reqBody := vo.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Tools:    tools,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.appConfig.AIAPIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API请求失败 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result vo.ChatCompletionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Error != nil {
		return nil, fmt.Errorf("API错误: %s", result.Error.Message)
	}

	return &result, nil
}

// ─────────────────────────────────────────
// WEBSEARCH LAYER
// ─────────────────────────────────────────

// executeWebSearch 执行搜索，优先 Tavily，次选萌娘百科（VN专项，非严格防剧透时），降级 DuckDuckGo
func (s *AiService) executeWebSearch(query string) string {
	// 尝试 Tavily
	if s.appConfig.TavilyAPIKey != "" {
		result, err := utils.SearchViaTavily(query, s.appConfig.TavilyAPIKey)
		if err == nil && result != "" {
			return result
		}
		applog.LogError(s.ctx, "[AIService] Tavily search failed: "+err.Error())
	}

	// 萌娘百科：VN/Galgame 专项，剧透等级为 none 时跳过（词条含大量剧情细节）
	if s.appConfig.AISpoilerLevel != "none" {
		result, err := utils.SearchViaMoeGirl(query)
		if err == nil && result != "" {
			return result
		}
	}

	// 降级 DuckDuckGo
	result, err := utils.SearchViaDuckDuckGo(query)
	if err == nil && result != "" {
		return result
	}

	return fmt.Sprintf("[WebSearch] 搜索「%s」失败，请AI依据本地数据进行分析。", query)
}
