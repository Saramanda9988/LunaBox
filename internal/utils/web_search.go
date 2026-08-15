package utils

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"lunabox/internal/utils/httputils"
	"lunabox/internal/utils/proxyutils"
	"resty.dev/v3"
)

const webSearchRetryCount = 3

// SearchViaTavily 使用 Tavily Search API
func SearchViaTavily(query string, apiKey string) (string, error) {
	return SearchViaTavilyWithProxyConfig(query, apiKey, nil)
}

func SearchViaTavilyWithProxyConfig(query string, apiKey string, proxyConfig proxyutils.ProxyConfigProvider) (string, error) {
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

	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:     15 * time.Second,
		ProxyConfig: proxyConfig,
	})
	if err != nil {
		return "", err
	}
	resp, err := newWebSearchRequest(client).
		SetRetryAllowNonIdempotent(true).
		SetHeader("Content-Type", "application/json").
		SetBody(jsonData).
		Post("https://api.tavily.com/search")
	if err != nil {
		return "", err
	}
	if err := webSearchResponseError("Tavily", resp); err != nil {
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
	if err := json.Unmarshal(resp.Bytes(), &tavilyResp); err != nil {
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
	return SearchViaDuckDuckGoWithProxyConfig(query, nil)
}

func SearchViaDuckDuckGoWithProxyConfig(query string, proxyConfig proxyutils.ProxyConfigProvider) (string, error) {
	ddgURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) + "&format=json&no_html=1&skip_disambig=1"
	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:     10 * time.Second,
		ProxyConfig: proxyConfig,
	})
	if err != nil {
		return "", err
	}
	resp, err := newWebSearchRequest(client).Get(ddgURL)
	if err != nil {
		return "", err
	}
	if err := webSearchResponseError("DuckDuckGo", resp); err != nil {
		return "", err
	}

	var ddgResp struct {
		AbstractText string `json:"AbstractText"`
		AbstractURL  string `json:"AbstractURL"`
		Heading      string `json:"Heading"`
	}
	if err := json.Unmarshal(resp.Bytes(), &ddgResp); err != nil {
		return "", err
	}
	if ddgResp.AbstractText == "" {
		return "", fmt.Errorf("no result")
	}

	return fmt.Sprintf("[WebSearch 结果 - 来源: DuckDuckGo] %s\n%s\n参考：%s",
		ddgResp.Heading, ddgResp.AbstractText, ddgResp.AbstractURL), nil
}

func SearchViaMoeGirl(query string) (string, error) {
	return SearchViaMoeGirlWithProxyConfig(query, nil)
}

func SearchViaMoeGirlWithProxyConfig(query string, proxyConfig proxyutils.ProxyConfigProvider) (string, error) {
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
	client, _, err := httputils.NewRestyClient(httputils.ClientOptions{
		Timeout:     10 * time.Second,
		ProxyConfig: proxyConfig,
	})
	if err != nil {
		return "", err
	}
	resp, err := newWebSearchRequest(client).Get(apiURL)
	if err != nil {
		return "", err
	}
	if err := webSearchResponseError("MoeGirl", resp); err != nil {
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
	if err := json.Unmarshal(resp.Bytes(), &moeResp); err != nil {
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

func newWebSearchRequest(client *resty.Client) *resty.Request {
	return client.R().
		SetRetryCount(webSearchRetryCount).
		AddRetryConditions(
			resty.RetryConditionStatusTooManyRequests,
			resty.RetryConditionStatus5XX,
		)
}

func webSearchResponseError(source string, response *resty.Response) error {
	if response == nil {
		return fmt.Errorf("%s search returned an empty response", source)
	}
	if response.StatusCode() >= 200 && response.StatusCode() < 300 {
		return nil
	}
	detail := strings.TrimSpace(response.String())
	if len(detail) > 512 {
		detail = detail[:512]
	}
	if detail == "" {
		return fmt.Errorf("%s search failed: HTTP %d", source, response.StatusCode())
	}
	return fmt.Errorf("%s search failed: HTTP %d: %s", source, response.StatusCode(), detail)
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
