package authdomain

import "time"

// UIPreferences 用户 UI 主题偏好（theme/palette/font 云端同步）。
// 值与前端 ThemeContext 白名单对应；数据库默认值即 DEFAULT_* 常量。
type UIPreferences struct {
	UserID    int64     `json:"user_id"`
	Theme     string    `json:"theme"`
	Palette   string    `json:"palette"`
	Font      string    `json:"font"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UIPreferencesDefaults 前端 ThemeContext 的默认值（保持同一真相源）。
const (
	DefaultUITheme   = "light"
	DefaultUIPalette = "forest"
	DefaultUIFont    = "sans"
)
