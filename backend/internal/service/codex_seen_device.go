package service

import "context"

// CodexSeenDevice 单个已观测设备（来自网关入口的 x-codex-installation-id）。
type CodexSeenDevice struct {
	DeviceID   string `json:"device_id"`
	LastSeenAt int64  `json:"last_seen_at"`
}

// CodexSeenDeviceEntry 全量列表条目（按设备去重）。
type CodexSeenDeviceEntry struct {
	DeviceID   string `json:"device_id"`
	LastSeenAt int64  `json:"last_seen_at"`
}

// CodexSeenDeviceRecorder 记录各 API Key 观测到的真实设备 ID，
// 供管理员为 Codex 指纹收敛（device 档）选择真机设备，避免手工抓包。
type CodexSeenDeviceRecorder interface {
	// Record 记录一次观测（异步语义：实现应保证失败不影响主请求）。
	Record(ctx context.Context, apiKeyID int64, deviceID string)
	// ListByAPIKey 返回该 Key 观测到的设备（按最后出现时间倒序）。
	ListByAPIKey(ctx context.Context, apiKeyID int64) ([]CodexSeenDevice, error)
	// ListAll 返回全部观测设备（按设备去重，时间倒序）。
	ListAll(ctx context.Context) ([]CodexSeenDeviceEntry, error)
	// Delete 移除单条观测记录。
	Delete(ctx context.Context, apiKeyID int64, deviceID string) error
}

// RecordCodexSeenDeviceIfPresent 在存在安装 ID 头时记录设备观测。
// 防御性封装：nil receiver / 空头静默跳过。
func RecordCodexSeenDeviceIfPresent(r CodexSeenDeviceRecorder, c interface{ GetHeader(string) string }, apiKeyID int64) {
	if r == nil || apiKeyID <= 0 {
		return
	}
	if c == nil {
		return
	}
	deviceID := c.GetHeader("x-codex-installation-id")
	if deviceID == "" {
		return
	}
	r.Record(context.Background(), apiKeyID, deviceID)
}
