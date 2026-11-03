package library

import (
	"lunabox/desktop-mygo/ui/locales"
)

type importOption struct {
	key, icon          string
	label, description func() string
}

// importOptions 对应 EMPTY_STATE_IMPORT_OPTIONS 与“添加游戏”菜单里的导入来源。
var importOptions = []importOption{
	{"potatovn", "", locales.LibraryImportPotatoVN, locales.LibraryImportPotatoVNDesc},
	{"playnite", "", locales.LibraryImportPlaynite, locales.LibraryImportPlayniteDesc},
	{"yukihub", "", locales.LibraryImportYukiHub, locales.LibraryImportYukiHubDesc},
	{"vnite", "", locales.LibraryImportVnite, locales.LibraryImportVniteDesc},
	{"reinamanager", "", locales.LibraryImportReinaManager, locales.LibraryImportReinaManagerDesc},
	{"steam", "steam", locales.LibraryImportSteam, locales.LibraryImportSteamDesc},
}
