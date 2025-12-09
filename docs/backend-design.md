## 实体类（Go Struct）

所有结构体用于数据库映射或 API 传输。

### 1. 用户
```go
type User struct {
    ID                 string    `json:"id"`
    CreatedAt          time.Time `json:"created_at"`
    DefaultBackupTarget string    `json:"default_backup_target"` // "s3" or "docker"
}
```

### 2. 游戏
```go
type Game struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    Name       string    `json:"name"`
    CoverURL   string    `json:"cover_url"`
    Company    string    `json:"company"`
    Summary    string    `json:"summary"`
    SourceType string    `json:"source_type"` // "local", "bangumi", "vndb"
    SourceID   string    `json:"source_id"`
    CachedAt   time.Time `json:"cached_at"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### 3. 游玩会话
```go
type PlaySession struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    GameID    string    `json:"game_id"`
    StartTime time.Time `json:"start_time"`
    EndTime   time.Time `json:"end_time"`
    Duration  int       `json:"duration"` // seconds
}
```

### 4. 分类
```go
type Category struct {
    ID       string `json:"id"`
    UserID   string `json:"user_id"`
    Name     string `json:"name"`
    IsSystem bool   `json:"is_system"`
}
```

### 5. 游戏-分类关联（仅用于内部，通常不直接暴露）
```go
type GameCategory struct {
    GameID      string `json:"game_id"`
    CategoryID  string `json:"category_id"`
}
```

### 6. 首页数据
```go
type HomePageData struct {
    RecentGames      []Game          `json:"recent_games"`
    RecentlyAdded    []Game          `json:"recently_added"`
    TodayPlayTimeSec int             `json:"today_play_time_sec"`
}
```

### 7. 统计数据
```go
type Stats struct {
    TotalPlayTimeSec     int    `json:"total_play_time_sec"`
    WeeklyPlayTimeSec    int    `json:"weekly_play_time_sec"`
    LongestGameID        string `json:"longest_game_id"`
    MostPlayedGameID     string `json:"most_played_game_id"`
    LongestGameName      string `json:"longest_game_name"`
    MostPlayedGameName   string `json:"most_played_game_name"`
}
```

### 8. 外部元数据请求
```go
type MetadataRequest struct {
    Type string `json:"type"` // "bangumi" or "vndb"
    ID   string `json:"id"`
}
```

### 9. AI 摘要请求
```go
type AISummaryRequest struct {
    ChatIDs []string `json:"chat_ids"`
}
```

---

## 后端可暴露的方法名

这些方法将被前端通过 `return window['go']['main']['XXX']['XXX'](arg1);` 调用。

### ▶ 内部数据管理
```go
// 游戏
GetGames() ([]Game, error)
GetGameByID(id string) (Game, error)
AddGame(game Game) error
UpdateGame(game Game) error

// 首页 & 统计
GetHomePageData() (HomePageData, error)
GetStats() (Stats, error)

// 分类
GetCategories() ([]Category, error)
AddCategory(name string) error
AddGameToCategory(gameID, categoryID string) error

// 游玩记录（由客户端上报）
ReportPlaySession(session PlaySession) error
```

### 外部集成
```go
// 代理获取第三方元数据（Bangumi/vnDB）
FetchMetadata(req MetadataRequest) (Game, error)

// 备份：返回 S3 预签名上传 URL
GetBackupPresignedURL(filename string) (string, error)

// AI 摘要
AISummarize(req AISummaryRequest) (string, error)
```

### 用户（单用户场景）
```go
GetOrCreateCurrentUser() (User, error)
```

---

> **说明**：
> - 所有方法**自动绑定当前用户**（Wails 后端在初始化时获取或创建唯一用户）
> - **无需传 `user_id`**，由后端在数据库操作时自动附加
> - **DuckDB 连接**在 Wails 后端 `NewApp()` 时初始化，所有方法共享, 退出时通过onShutDown钩子退出

---
