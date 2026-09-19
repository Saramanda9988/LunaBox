package reinamanager

// Data aggregates the games read from a ReinaManager SQLite backup.
type Data struct {
	Games []Game
}

// PlayStatus 对应 ReinaManager games.clear 字段的游玩状态。
type PlayStatus int64

const (
	PlayStatusWish    PlayStatus = 1
	PlayStatusPlayed  PlayStatus = 2
	PlayStatusPlaying PlayStatus = 3
	PlayStatusOnHold  PlayStatus = 4
	PlayStatusDropped PlayStatus = 5
)

// Game represents a game record and its related ReinaManager data.
type Game struct {
	ID                int64
	IDType            string
	Date              string
	LocalPath         string
	Executable        string
	SavePath          string
	Clear             PlayStatus
	UseLocaleEmulator bool
	UseMagpie         bool
	Custom            CustomData
	CreatedAt         int64
	UpdatedAt         int64
	Sources           map[string]Source
	Sessions          []Session
}

type Source struct {
	Source     string
	ExternalID string
	Data       Metadata
}

type Metadata struct {
	Image       string   `json:"image"`
	Name        string   `json:"name"`
	NameCN      string   `json:"name_cn"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags"`
	Developer   string   `json:"developer"`
	Score       *float64 `json:"score"`
	ReleaseDate string   `json:"date"`
	NSFW        *bool    `json:"nsfw"`
}

type CustomData struct {
	Image       string   `json:"image"`
	CoverSource string   `json:"cover_source"`
	Name        string   `json:"name"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags"`
	Developer   string   `json:"developer"`
	NSFW        *bool    `json:"nsfw"`
	UserRating  *float64 `json:"user_rating"`
}

type Session struct {
	StartTime int64
	EndTime   int64
	Duration  int64
}
