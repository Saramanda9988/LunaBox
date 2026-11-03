package locales

// renames 是拍平后会与别的 key 重名的 key 的新名字（ARB 里的消息名），[Messages.Key] 按它查找。
// 在 ARB 里新增这样的消息时，在这里加上对应的一项。
var renames = map[string]string{
	"settings.metadata.idPreview.loading": "settings_metadata_idPreview_loadingText",
}
