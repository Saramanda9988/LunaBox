package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// SearchViaTavily 使用 Tavily Search API
func SearchViaTavily(query string, apiKey string) (string, error) {
	payload := map[string]interface{}{
		"api_key":        apiKey,
		"query":          query,
		"search_depth":   "basic",
		"max_results":    3,
		"include_answer": true,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.tavily.com/search", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tavilyResp struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
			URL     string `json:"url"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[WebSearch 结果 - 来源: Tavily] 搜索：%s\n", query))
	if tavilyResp.Answer != "" {
		sb.WriteString(fmt.Sprintf("摘要：%s\n\n", tavilyResp.Answer))
	}
	for i, r := range tavilyResp.Results {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("- %s\n  %s\n", r.Title, r.Content))
	}
	return sb.String(), nil
}

// SearchViaDuckDuckGo 使用 DuckDuckGo Instant Answer API（免费，无需 Key）
func SearchViaDuckDuckGo(query string) (string, error) {
	ddgURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) + "&format=json&no_html=1&skip_disambig=1"
	req, err := http.NewRequest("GET", ddgURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LunaBox/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ddgResp struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Heading      string `json:"Heading"`
	}
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return "", err
	}
	if ddgResp.AbstractText == "" {
		return "", fmt.Errorf("no result")
	}

	return fmt.Sprintf("[WebSearch 结果 - 来源: DuckDuckGo] %s\n%s\n参考：%s",
		ddgResp.Heading, ddgResp.AbstractText, ddgResp.AbstractURL), nil
}

func SearchViaMoeGirl(query string) (string, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("format", "json")
	params.Set("generator", "search")
	params.Set("gsrsearch", query)
	params.Set("gsrlimit", "1")
	params.Set("gsrnamespace", "0")
	params.Set("prop", "extracts")
	// 不设 exintro，取多个段落（简介 + 性格 + 部分章节），exchars 限制总量
	params.Set("explaintext", "1")        // 请求纯文本（ruby/span 模板可能残留 HTML，见 cleanMoeGirlHTML）
	params.Set("exchars", "2400")         // API 层面字符上限（最大值）
	params.Set("exsectionformat", "wiki") // 保留 == 章节名 == 便于 AI 识别段落边界

	apiURL := "https://zh.moegirl.org.cn/api.php?" + params.Encode()
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LunaBox/1.0 (game library app)")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var moeResp struct {
		Query struct {
			Pages map[string]struct {
				Title   string `json:"title"`
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &moeResp); err != nil {
		return "", err
	}
	if len(moeResp.Query.Pages) == 0 {
		return "", fmt.Errorf("moegirl: no results for %q", query)
	}

	var title, extract string
	for _, page := range moeResp.Query.Pages {
		title = page.Title
		extract = page.Extract
		break
	}
	if extract == "" {
		return "", fmt.Errorf("moegirl: empty extract for %q", title)
	}

	clean := cleanMoeGirlHTML(extract)
	if clean == "" {
		return "", fmt.Errorf("moegirl: extract empty after cleaning")
	}
	return fmt.Sprintf("[WebSearch 结果 - 来源: 萌娘百科] 词条：%s\n%s\n参考：https://zh.moegirl.org.cn/%s",
		title, clean, url.PathEscape(title)), nil
}

// cleanMoeGirlHTML 清洗萌娘百科 extract 中残留的 HTML。
// explaintext 参数并不能完全移除 ruby 注音和剧透 span，需手动处理。
var (
	// <rt ...>...</rt>：假名注音，整段移除
	moeRubyRT = regexp.MustCompile(`(?s)<rt[^>]*>.*?</rt>`)
	// <span title="...">...</span>：萌娘惯用剧透标记（title="你知道的太多了"）
	moeSpoilerSpan = regexp.MustCompile(`(?s)<span\s[^>]*title=["'][^"']*["'][^>]*>.*?</span>`)
	// 其余所有 HTML 标签
	moeHTMLTag = regexp.MustCompile(`<[^>]+>`)
	// 连续 3+ 空行压缩为 2 行
	moeMultiNL = regexp.MustCompile(`\n{3,}`)
)

func cleanMoeGirlHTML(raw string) string {
	raw = moeRubyRT.ReplaceAllString(raw, "")
	raw = moeSpoilerSpan.ReplaceAllString(raw, "")
	raw = moeHTMLTag.ReplaceAllString(raw, "")
	raw = moeMultiNL.ReplaceAllString(raw, "\n\n")
	return strings.TrimSpace(raw)
}
