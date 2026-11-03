package locales

import (
	"strings"

	"github.com/egoist/mygo/i18n"
)

// Key 按原版前端的 key（"common.playing"）在应用当前的语言里说出消息，args 是成对的参数名与值
// （"count", "3"）。原版用变量拼 key 的地方（t(`gameStatus.${status}`)）用它；固定的 key 用生成的函数，
// 编译期就能发现拼错。key 不存在时返回 key 本身，与 i18next 相同。
func Key(key string, args ...string) string {
	return In(i18n.Locale()).Key(key, args...)
}

// Key 按原版前端的 key 在 msgs 的语言里说出消息，见 [Key]。
func (msgs Messages) Key(key string, args ...string) string {
	name, ok := renames[key]
	if !ok {
		name = strings.ReplaceAll(key, ".", "_")
	}
	var values map[string]any
	if len(args) > 0 {
		values = make(map[string]any, len(args)/2)
		for i := 0; i+1 < len(args); i += 2 {
			values[args[i]] = args[i+1]
		}
	}
	if s := Catalog.Format(msgs.locale, name, values); s != name {
		return s
	}
	return key // Catalog.Format 找不到消息时返回 name
}
