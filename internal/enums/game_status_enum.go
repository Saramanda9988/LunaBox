package enums

type GameStatus string

const (
	StatusNotStarted GameStatus = "not_started" // 未开始
	StatusPlaying    GameStatus = "playing"     // 游玩中
	StatusCompleted  GameStatus = "completed"   // 已通关
	StatusOnHold     GameStatus = "on_hold"     // 搁置
)

var AllGameStatuses = []struct {
	Value  GameStatus
	TSName string
}{
	{StatusNotStarted, "NOT_STARTED"},
	{StatusPlaying, "PLAYING"},
	{StatusCompleted, "COMPLETED"},
	{StatusOnHold, "ON_HOLD"},
}
