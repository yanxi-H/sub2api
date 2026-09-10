package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// Codex 设备观测记录：网关入口捕获各 API Key 真实使用的
// x-codex-installation-id，供管理员在账号指纹收敛（device 档）时
// 直接选择真机设备 ID，避免手工抓包。
//
// 存储：每个 api_key 一个 ZSet（member=deviceID, score=最后见到的时间戳），
// ZADD 以 GT 语义只前进不回退；查询时 ZREMRANGEBYSCORE 清理过期后返回倒序列表。

const (
	codexSeenDeviceKeyPrefix = "codex:seen_device:api_key:"
	// 保留 90 天：设备 ID 是稳定标识，长窗口便于低频设备也能被选到。
	codexSeenDeviceTTL         = 90 * 24 * time.Hour
	codexSeenDeviceMaxPerKey   = 20
	codexSeenDeviceScoreLayout = "20060102150405"
)

type codexSeenDeviceRepo struct {
	rdb *redis.Client
}

// ListAll 返回全部 Key 观测到的设备（按 device 去重，聚合来源 Key 与最后时间）。
func (r *codexSeenDeviceRepo) ListAll(ctx context.Context) ([]service.CodexSeenDeviceEntry, error) {
	if r == nil || r.rdb == nil {
		return nil, nil
	}
	type agg struct {
		apiKeyIDs map[int64]struct{}
		lastSeen  int64
	}
	byDevice := map[string]*agg{}
	var cursor uint64
	for {
		keys, next, err := r.rdb.Scan(ctx, cursor, codexSeenDeviceKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			apiKeyID := int64(0)
			fmt.Sscanf(key, codexSeenDeviceKeyPrefix+"%d", &apiKeyID)
			entries, err := r.rdb.ZRevRangeWithScores(ctx, key, 0, -1).Result()
			if err != nil {
				continue
			}
			for _, e := range entries {
				deviceID, _ := e.Member.(string)
				if deviceID == "" {
					continue
				}
				seen := int64(e.Score)
				a, ok := byDevice[deviceID]
				if !ok {
					a = &agg{apiKeyIDs: map[int64]struct{}{}}
					byDevice[deviceID] = a
				}
				a.apiKeyIDs[apiKeyID] = struct{}{}
				if seen > a.lastSeen {
					a.lastSeen = seen
				}
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	out := make([]service.CodexSeenDeviceEntry, 0, len(byDevice))
	for deviceID, a := range byDevice {
		out = append(out, service.CodexSeenDeviceEntry{
			DeviceID:   deviceID,
			LastSeenAt: a.lastSeen,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt > out[j].LastSeenAt })
	return out, nil
}

// Delete 移除单条观测记录。
func (r *codexSeenDeviceRepo) Delete(ctx context.Context, apiKeyID int64, deviceID string) error {
	if r == nil || r.rdb == nil || apiKeyID <= 0 || deviceID == "" {
		return nil
	}
	return r.rdb.ZRem(ctx, codexSeenDeviceKey(apiKeyID), deviceID).Err()
}

func NewCodexSeenDeviceRepo(rdb *redis.Client) *codexSeenDeviceRepo {
	return &codexSeenDeviceRepo{rdb: rdb}
}

func codexSeenDeviceKey(apiKeyID int64) string {
	return fmt.Sprintf("%s%d", codexSeenDeviceKeyPrefix, apiKeyID)
}

// Record 记录一次设备观测（幂等；重复设备只前进时间戳）。实现 service.CodexSeenDeviceRecorder。
func (r *codexSeenDeviceRepo) Record(ctx context.Context, apiKeyID int64, deviceID string) {
	if r == nil || r.rdb == nil || apiKeyID <= 0 || deviceID == "" {
		return
	}
	key := codexSeenDeviceKey(apiKeyID)
	score := float64(time.Now().Unix())
	pipe := r.rdb.Pipeline()
	pipe.ZAdd(ctx, key, redis.Z{Score: score, Member: deviceID})
	pipe.ZRemRangeByRank(ctx, key, 0, -(codexSeenDeviceMaxPerKey + 1))
	pipe.Expire(ctx, key, codexSeenDeviceTTL)
	_, _ = pipe.Exec(ctx)
}

// CodexSeenDevice 单个已观测设备。
type CodexSeenDevice struct {
	DeviceID   string `json:"device_id"`
	LastSeenAt int64  `json:"last_seen_at"` // unix 秒
}

// ListByAPIKey 返回该 Key 观测到的设备（按最后出现时间倒序）。
func (r *codexSeenDeviceRepo) ListByAPIKey(ctx context.Context, apiKeyID int64) ([]service.CodexSeenDevice, error) {
	if r == nil || r.rdb == nil || apiKeyID <= 0 {
		return nil, nil
	}
	key := codexSeenDeviceKey(apiKeyID)
	cutoff := float64(time.Now().Add(-codexSeenDeviceTTL).Unix())
	if err := r.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("(%f", cutoff)).Err(); err != nil {
		return nil, err
	}
	entries, err := r.rdb.ZRevRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]service.CodexSeenDevice, 0, len(entries))
	for _, e := range entries {
		deviceID, _ := e.Member.(string)
		if deviceID == "" {
			continue
		}
		out = append(out, service.CodexSeenDevice{
			DeviceID:   deviceID,
			LastSeenAt: int64(e.Score),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeenAt > out[j].LastSeenAt })
	return out, nil
}
