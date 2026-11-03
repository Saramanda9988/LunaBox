// Package locales 是界面的多语言消息：每种语言一个 ARB 目录（zh-CN.json 等），直接编辑；
// catalog_gen.go 是 mygo generate 写出的类型化函数，改了 ARB 后运行 go tool mygo generate 重新生成。
//
// 每个函数用应用当前的语言（mygo.App.Locale）说出消息；没有运行 App 时（测试）用源语言 zh-CN。
package locales
