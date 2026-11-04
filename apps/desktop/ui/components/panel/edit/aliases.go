package edit

import (
	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
	"lunabox/desktop/ui/components/gamefield"
	"lunabox/desktop/ui/locales"
)

// aliasesField 对应“游戏别名”：标题、gamefield.Aliases 与说明。
func (e *editor) aliasesField(c *ui.Context) {
	t, s := e.t, e.s
	ui.Column(c.Key("aliases-field")).Children(func() {
		gamefield.Label(c, t, locales.GameEditAliases())
		gamefield.Aliases(c, t, "aliases", gamefield.AliasesProps{
			Aliases: e.g.Aliases, Adding: &s.aliasAdding, Draft: &s.aliasDraft,
			OnChange: func(aliases []string) { e.set(func(g *models.Game) { g.Aliases = aliases }) },
		})
		gamefield.Hint(c, t, locales.GameEditAliasHint(), false)
	})
}
