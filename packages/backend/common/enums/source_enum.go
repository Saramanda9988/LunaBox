package enums

type SourceType string

const (
	Local      SourceType = "local"
	Bangumi    SourceType = "bangumi"
	VNDB       SourceType = "vndb"
	Ymgal      SourceType = "ymgal"
	Steam      SourceType = "steam"
	DLsite     SourceType = "dlsite"
	TouchGal   SourceType = "touchgal"
	Hikarinagi SourceType = "hikarinagi"
	Kungal     SourceType = "kungal"

	ErogameScape SourceType = "erogamescape"
)

var AllSourceTypes = []struct {
	Value  SourceType
	TSName string
}{
	{Local, "LOCAL"},
	{Bangumi, "BANGUMI"},
	{VNDB, "VNDB"},
	{Ymgal, "YMGAL"},
	{Steam, "STEAM"},
	{DLsite, "DLSITE"},
	{TouchGal, "TOUCHGAL"},
	{Hikarinagi, "HIKARINAGI"},
	{Kungal, "KUNGAL"},
	{ErogameScape, "EROGAMESCAPE"},
}

// MetadataSourceTypes 是全部元数据来源（不含 Local），顺序同原版 ALL_METADATA_SOURCES。
// 新增来源只在这里加类型；后端校验与界面列表（apps/desktop/ui/metasource）都从这里取。
var MetadataSourceTypes = []SourceType{
	Bangumi, VNDB, Ymgal, DLsite, TouchGal, Hikarinagi, Kungal, ErogameScape, Steam,
}
