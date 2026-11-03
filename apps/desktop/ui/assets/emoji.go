package assets

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed emoji.json
var emojiJSON []byte

// Emoji 是 emoji-picker-element 的一个表情（数据由 tools/emoji/fetch-emoji.mjs 生成）。
type Emoji struct {
	Unicode    string
	Group      int // emojibase 的分组：0 笑脸 1 人物 3 动物 4 食物 5 旅行 6 活动 7 物品 8 符号 9 旗帜
	Annotation string
	Tags       []string
	Skins      []string // 五种肤色（浅 → 深）的变体，没有时为空
}

// Emojis 按 emojibase 的顺序返回全部表情。
var Emojis = sync.OnceValue(func() []Emoji {
	var rows [][]json.RawMessage
	if err := json.Unmarshal(emojiJSON, &rows); err != nil {
		panic(err)
	}
	out := make([]Emoji, 0, len(rows))
	for _, r := range rows {
		var e Emoji
		_ = json.Unmarshal(r[0], &e.Unicode)
		_ = json.Unmarshal(r[1], &e.Group)
		_ = json.Unmarshal(r[2], &e.Annotation)
		_ = json.Unmarshal(r[3], &e.Tags)
		if len(r) > 4 {
			_ = json.Unmarshal(r[4], &e.Skins)
		}
		out = append(out, e)
	}
	return out
})
