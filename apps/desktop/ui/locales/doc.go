// Package locales 是界面的多语言消息：每种语言一个 ARB 目录（zh-CN.json 等），由 tools/i18n 从原版前端的
// apps/desktop/frontend/src/locales 转换而来，catalog_gen.go 是 mygo generate 写出的类型化函数。
// 这些文件都是生成的，不要手改：改原版的语言包（或 tools/i18n/extra），再运行 go run ./tools/i18n。
//
// 每个函数用应用当前的语言（mygo.App.Locale）说出消息；没有运行 App 时（测试）用源语言 zh-CN。
package locales
