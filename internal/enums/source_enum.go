package enums

type SourceType string

const (
	Local   SourceType = "local"
	Bangumi SourceType = "bangumi"
	VNDB    SourceType = "vndb"
	Ymgal   SourceType = "ymgal"
)

var AllSourceTypes = []struct {
	Value  SourceType
	TSName string
}{
	{Local, "LOCAL"},
	{Bangumi, "BANGUMI"},
	{VNDB, "VNDB"},
	{Ymgal, "YMGAL"},
}
