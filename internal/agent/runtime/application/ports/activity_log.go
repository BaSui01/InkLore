package ports

import (
	"context"
	"time"
)

// ActivityLogRecord 是一次运行的**人类可读**活动摘要。
//
// 与 trace_events 的区别：trace_events 是逐事件的机器可读审计流，活动日志是
// 「这次运行到底做了什么」的一句话摘要，用于快速排查与前端轨迹列表（INK-037）。
type ActivityLogRecord struct {
	NodeID  string
	LogPath string
	Summary string
	Content string
	RunID   int64
	AgentID int64
}

// ActivityLog 是读回的活动日志。
//
// 字段带 json 标签：该结构直接作为 `GET /v1/agents/runs/:runID/activity` 的响应元素，
// 不能把 Go 字段名暴露成对外契约（INK-041）。
type ActivityLog struct {
	CreatedAt time.Time `json:"created_at"`
	NodeID    string    `json:"node_id"`
	LogPath   string    `json:"log_path"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	RunID     int64     `json:"run_id"`
	ID        int64     `json:"id"`
}

// ActivityLogStore 记录并读取运行活动日志。
//
// 约定：写入必须是**尽力而为**的 —— 活动日志是审计增强项，写失败只告警，
// 绝不能把一次成功的运行变成失败。
type ActivityLogStore interface {
	Log(ctx context.Context, record ActivityLogRecord) error
	ListByRunID(ctx context.Context, runID int64) ([]ActivityLog, error)
}
