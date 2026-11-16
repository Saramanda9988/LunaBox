// Package panel 对应原版 components/panel 里游戏详情页的标签页。每个面板一个子包（edit、launch、backup、
// progress、review），互不依赖；这里只放它们共用的输入。面板自己调用 Env.Svc 里的真实服务，
// 与原版面板直接调用 bindings 一样。
package panel

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop/ui/components"
	"lunabox/desktop/ui/env"
)

// Game 是详情页传给标签页的内容，对应原版面板的 game、onGameChange、onDelete 等属性。
type Game struct {
	Env  *env.Env
	Game *models.Game // 页面持有的当前游戏；面板不要直接改它，改动通过 OnChange 提交

	// OnChange 对应 updateGameState：替换页面里的游戏，500ms 后自动保存（UpdateGame），离开页面前补存。
	OnChange func(models.Game)
	// OnSaved 在面板自己调用了会改游戏的服务（如更新元数据、换封面）之后调用：页面重新读取游戏并刷新游戏库。
	OnSaved func()
	// OnDeleted 在游戏被删除后调用：页面回到游戏库并刷新列表。
	OnDeleted func()

	// Tags 是页面里合并了同名标签的游戏标签；OnAddTag 添加用户标签，OnDeleteTags 删除其中几个（同名的几个来源一起删），
	// 都由页面更新标签并提示。
	Tags         []components.TagGroup
	OnAddTag     func(c *ui.Context, name string)
	OnDeleteTags func(c *ui.Context, groups []components.TagGroup)
}

// TopOffset 是弹窗遮罩的起点（原版的 #app-modal-root 在顶栏下方）。
const TopOffset = env.TopbarHeight
