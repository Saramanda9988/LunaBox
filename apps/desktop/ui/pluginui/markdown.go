package pluginui

import (
	"os"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"lunabox/desktop/ui/theme"
)

// markdownState 是一个 Markdown 原语跨帧保留的 markdown.Options：它带着解析与排版缓存，
// 每帧新建会让整段文字每帧重新排版。明暗主题或字体变化时重建。
type markdownState struct {
	opts      *markdown.Options
	dark      bool
	uiFont    string
	monoFont  string
	linkPress string
}

// renderMarkdown 用 MyGo 的 ui/markdown 渲染插件的 Markdown：CommonMark 与 GitHub 扩展（表格、任务列表、删除线、
// 自动链接）、代码高亮与复制、选择与复制为 Markdown。streaming 为 true 时按流式回复显示：
// 只解析新追加的部分，未写完的语法（**加粗、未闭合的代码块）先按最终样子显示，文字逐词淡入。
func (m *Manager) renderMarkdown(c *ui.Context, t *theme.Theme, inst *Instance, key, content, linkPressID string, streaming bool) ui.Element {
	if inst.Markdowns == nil {
		inst.Markdowns = make(map[string]*markdownState)
	}
	uiFont, monoFont := theme.ActiveUIFont(), theme.ActiveMonoFont()
	st := inst.Markdowns[key]
	if st == nil || st.dark != t.Dark || st.uiFont != uiFont || st.monoFont != monoFont {
		st = &markdownState{
			opts:     &markdown.Options{Theme: pluginMarkdownTheme(t, uiFont, monoFont), LoadImage: m.loadMarkdownImage},
			dark:     t.Dark,
			uiFont:   uiFont,
			monoFont: monoFont,
		}
		inst.Markdowns[key] = st
	}
	// 插件传了 onLinkPress 时链接交给插件处理；否则按 MyGo 的默认行为打开 http/https/mailto 链接
	if st.opts.OnLink == nil || st.linkPress != linkPressID {
		st.linkPress = linkPressID
		if linkPressID == "" {
			st.opts.OnLink = nil
		} else {
			id := linkPressID
			st.opts.OnLink = func(url string) { m.dispatch(inst, id, url) }
		}
	}
	ctx := c.Key(key)
	if streaming {
		return markdown.Stream(ctx, content, st.opts)
	}
	return markdown.View(ctx, content, st.opts)
}

func pluginMarkdownTheme(t *theme.Theme, uiFont, monoFont string) *markdown.Theme {
	step := theme.TextSM
	th := &markdown.Theme{
		FontSize:            step.Size,
		Font:                uiFont,
		CodeFont:            monoFont,
		LineHeight:          step.Line / step.Size,
		Text:                t.Text,
		Strong:              t.TextStrong,
		Muted:               t.TextMuted,
		Link:                t.Accent,
		Code:                t.TextStrong,
		CodeBackground:      t.Text.Alpha(0.08),
		CodeBlockBackground: t.Pick(theme.Brand100, theme.Brand900),
		CodeBlockBorder:     t.Divider,
		BlockSpacing:        0.75,
		ItemSpacing:         0.25,
		ListIndent:          1.4,
		Rule:                t.Divider,
		QuoteText:           t.TextMuted,
	}
	sizes := [6]float32{1.45, 1.25, 1.1, 1, 1, 1}
	for l := range th.Headings {
		th.Headings[l] = markdown.HeadingStyle{Size: sizes[l], Weight: theme.WeightBold, Color: t.TextStrong, Top: 1, Bottom: 0.4}
	}
	return th
}

// loadMarkdownImage 在 ui/markdown 的后台 goroutine 上加载 Markdown 里的图片：本地路径（含 /local/...）直接读取，
// http(s) 走插件图片的磁盘缓存与下载。
func (m *Manager) loadMarkdownImage(url string) (ui.ImageSource, error) {
	if path, local := m.resolveLocalPath(url); local {
		if path == "" {
			return nil, os.ErrNotExist
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return ui.DecodeImage(data)
	}
	disk := m.getDiskCache()
	if disk != nil {
		if data, ok := disk.loadBytes(url); ok {
			return ui.DecodeImage(data)
		}
	}
	data, err := m.fetchRemoteBytes(url)
	if err != nil {
		return nil, err
	}
	if disk != nil {
		disk.storeBytes(url, data)
	}
	return ui.DecodeImage(data)
}
