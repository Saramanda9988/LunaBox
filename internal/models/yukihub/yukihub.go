package yukihub

// Backup represents a YukiHub backup archive payload.
type Backup struct {
	App           string          `json:"app"`
	Schema        int             `json:"schema"`
	CreatedAt     int64           `json:"created_at"`
	Settings      BackupSettings  `json:"settings"`
	Games         []Game          `json:"games"`
	PlaySessions  []PlaySession   `json:"play_sessions"`
	MetadataCache []MetadataCache `json:"metadata_cache"`
}

type BackupSettings struct {
	MetadataSource string `json:"metadata_source"`
}

type Game struct {
	LocalID       int64  `json:"local_id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
	PlayStatus    string `json:"play_status"`
	NSFW          bool   `json:"nsfw"`
	TotalPlayTime int64  `json:"total_play_time"`
	LastPlayedAt  int64  `json:"last_played_at"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type PlaySession struct {
	SessionUUID string `json:"session_uuid"`
	GameLocalID int64  `json:"game_local_id"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	Duration    int64  `json:"duration"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type MetadataCache struct {
	GameLocalID int64  `json:"game_local_id"`
	Source      string `json:"source"`
	SourceID    string `json:"source_id"`
	JSON        string `json:"json"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Metadata struct {
	ID                    string   `json:"id"`
	ChineseTitle          string   `json:"chineseTitle"`
	OriginalTitle         string   `json:"originalTitle"`
	RomanTitle            string   `json:"romanTitle"`
	CoverURL              string   `json:"coverUrl"`
	Description           string   `json:"description"`
	TranslatedDescription string   `json:"translatedDescription"`
	Released              string   `json:"released"`
	Developer             string   `json:"developer"`
	TagsText              string   `json:"tagsText"`
	RatingText            string   `json:"ratingText"`
	CoverSexual           int      `json:"coverSexual"`
	ScreenshotURLs        []string `json:"screenshotUrls"`
}
