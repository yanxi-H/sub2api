package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

// 并发控制缓存常量定义
//
// 性能优化说明：
// 原实现使用 SCAN 命令遍历独立的槽位键（concurrency:account:{id}:{requestID}），
// 在高并发场景下 SCAN 需要多次往返，且遍历大量键时性能下降明显。
//
// 新实现改用 Redis 有序集合（Sorted Set）：
// 1. 每个账号/用户只有一个键，成员为 requestID，分数为时间戳
// 2. 使用 ZCARD 原子获取并发数，时间复杂度 O(1)
// 3. 使用 ZREMRANGEBYSCORE 清理过期槽位，避免手动管理 TTL
// 4. 单次 Redis 调用完成计数，减少网络往返
const (
	// 并发槽位键前缀（有序集合）
	// 格式: concurrency:account:{accountID}
	accountSlotKeyPrefix = "concurrency:account:"
	// 格式: concurrency:user:{userID}
	userSlotKeyPrefix = "concurrency:user:"
	// 格式: concurrency:api_key:{apiKeyID}
	apiKeySlotKeyPrefix      = "concurrency:api_key:"
	liveAccountSlotKeyPrefix = "concurrency:live:account:"
	liveUserSlotKeyPrefix    = "concurrency:live:user:"
	liveAPIKeySlotKeyPrefix  = "concurrency:live:api_key:"
	// API-key-scoped client WebSocket ingress leases use a shorter TTL than
	// ordinary request slots, because idle ingress sessions do not hold a turn slot.
	openAIWSIngressLeaseKeyPrefix  = "concurrency:openai_ws_ingress:api_key:"
	openAIWSIngressLeaseTTLSeconds = 60
	liveLeaseTTLSeconds            = 60
	// 等待队列计数器格式: concurrency:wait:{userID}
	waitQueueKeyPrefix = "concurrency:wait:"
	// 账号级等待队列计数器格式: wait:account:{accountID}
	accountWaitKeyPrefix = "wait:account:"
	// Request-body lane keys share one hash tag so the atomic scope/user
	// acquire script remains Redis Cluster compatible and the per-user active
	// limit can span both the heavy and recovery lanes.
	requestBodyLaneKeyPrefix     = "concurrency:request_body:"
	requestBodyLaneWaitKeyPrefix = "wait:request_body:"

	// 默认槽位过期时间（分钟），可通过配置覆盖
	defaultSlotTTLMinutes = 15
	// Admission leases are refreshed while a request is alive. Keeping this
	// independent from the general concurrency TTL makes crash recovery fast.
	requestBodyAdmissionLeaseTTLSeconds = 60

	// 活跃索引用来替代后台任务全量 SCAN 槽位键。
	// member 是账号/用户 ID，score 是“预计仍需关注到”的 Redis Unix 秒时间戳。
	accountActiveIndexKey     = "concurrency:account:active_index"      // ZSET member=accountID, score=expireAtUnixSeconds
	userActiveIndexKey        = "concurrency:user:active_index"         // ZSET member=userID, score=expireAtUnixSeconds
	requestBodyActiveIndexKey = "concurrency:request_body:active_index" // ZSET member=userID, score=expireAtUnixSeconds

	// 后台清理只按批处理索引候选，避免单次任务占用 Redis 太久。
	activeIndexCleanupBatchSize  = 1000
	activeIndexPipelineChunkSize = 500

	// 一次性迁移 marker：活跃索引机制上线前遗留的等待计数键无法被索引发现，
	// 且有流量时 TTL 会被不断刷新，必须清扫一次。marker 存在即代表已完成。
	legacyWaitSweepMarkerKey = "concurrency:startup:legacy_wait_sweep:v1"
)

var (
	// acquireScript 使用有序集合计数并在未达上限时添加槽位
	// 使用 Redis TIME 命令获取服务器时间，避免多实例时钟不同步问题
	// KEYS[1] = 普通槽位键，KEYS[2] = 对应 Live 槽位键
	// ARGV[1] = maxConcurrency
	// ARGV[2] = TTL（秒）
	// ARGV[3] = requestID
	// 返回 {是否成功, Redis 当前秒, 普通与 Live 总槽位数}。
	acquireScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local key = KEYS[1]
		local liveKey = KEYS[2]
		local maxConcurrency = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local requestID = ARGV[3]

		-- 使用 Redis 服务器时间，确保多实例时钟一致
		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		-- 清理过期槽位
		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		redis.call('ZREMRANGEBYSCORE', liveKey, '-inf', now - 60)

		-- 检查是否已存在（支持重试场景刷新时间戳）
		local exists = redis.call('ZSCORE', key, requestID)
		if exists ~= false then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return {1, now, redis.call('ZCARD', key) + redis.call('ZCARD', liveKey)}
		end

		-- 检查是否达到并发上限
		local count = redis.call('ZCARD', key) + redis.call('ZCARD', liveKey)
		if count < maxConcurrency then
			redis.call('ZADD', key, now, requestID)
			redis.call('EXPIRE', key, ttl)
			return {1, now, redis.call('ZCARD', key) + redis.call('ZCARD', liveKey)}
		end

		return {0, now, count}
	`)

	// getCountScript 统计有序集合中的槽位数量并清理过期条目
	// 使用 Redis TIME 命令获取服务器时间
	// KEYS[1] = 普通槽位键，KEYS[2] = 对应 Live 槽位键
	// ARGV[1] = TTL（秒）
	getCountScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local key = KEYS[1]
		local liveKey = KEYS[2]
		local ttl = tonumber(ARGV[1])

		-- 使用 Redis 服务器时间
		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		redis.call('ZREMRANGEBYSCORE', liveKey, '-inf', now - 60)
		return redis.call('ZCARD', key) + redis.call('ZCARD', liveKey)
	`)

	acquireLiveLeaseScript = redis.NewScript(`
		redis.replicate_commands()
		local accountRegular = KEYS[1]
		local accountLive = KEYS[2]
		local userRegular = KEYS[3]
		local userLive = KEYS[4]
		local apiLive = KEYS[5]
		local accountMax = tonumber(ARGV[1])
		local userMax = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])
		local leaseID = ARGV[4]
		local replacing = tonumber(ARGV[5])
		local now = tonumber(redis.call('TIME')[1])
		local liveExpireBefore = now - ttl
		redis.call('ZREMRANGEBYSCORE', accountLive, '-inf', liveExpireBefore)
		redis.call('ZREMRANGEBYSCORE', userLive, '-inf', liveExpireBefore)
		redis.call('ZREMRANGEBYSCORE', apiLive, '-inf', liveExpireBefore)
		if redis.call('ZSCORE', accountLive, leaseID) ~= false then
			return 1
		end
		local accountCount = redis.call('ZCARD', accountRegular) + redis.call('ZCARD', accountLive)
		local userCount = redis.call('ZCARD', userRegular) + redis.call('ZCARD', userLive)
		local allowance = 0
		if replacing == 1 then allowance = 1 end
		if accountMax > 0 and accountCount >= accountMax + allowance then return 0 end
		if userMax > 0 and userCount >= userMax + allowance then return 0 end
		redis.call('ZADD', accountLive, now, leaseID)
		redis.call('ZADD', userLive, now, leaseID)
		redis.call('ZADD', apiLive, now, leaseID)
		redis.call('EXPIRE', accountLive, ttl)
		redis.call('EXPIRE', userLive, ttl)
		redis.call('EXPIRE', apiLive, ttl)
		return 1
	`)

	refreshLiveLeaseScript = redis.NewScript(`
		redis.replicate_commands()
		local ttl = tonumber(ARGV[1])
		local leaseID = ARGV[2]
		local now = tonumber(redis.call('TIME')[1])
		local expireBefore = now - ttl
		for _, key in ipairs(KEYS) do
			redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
			if redis.call('ZSCORE', key, leaseID) == false then return 0 end
		end
		for _, key in ipairs(KEYS) do
			redis.call('ZADD', key, now, leaseID)
			redis.call('EXPIRE', key, ttl)
		end
		return 1
	`)

	// trackSlotScript 记录 stats-only 槽位，不做并发上限判断。
	// KEYS[1] = 有序集合键
	// ARGV[1] = TTL（秒）
	// ARGV[2] = requestID
	trackSlotScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local key = KEYS[1]
		local ttl = tonumber(ARGV[1])
		local requestID = ARGV[2]

		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl

		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		redis.call('ZADD', key, now, requestID)
		redis.call('EXPIRE', key, ttl)
		return 1
	`)

	// trackUserSlotStateScript records an unlimited user's request for stats only
	// and returns the observed count plus Redis time.
	trackUserSlotStateScript = redis.NewScript(`
		redis.replicate_commands()
		local key = KEYS[1]
		local ttl = tonumber(ARGV[1])
		local requestID = ARGV[2]
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', key, '-inf', now - ttl)
		redis.call('ZADD', key, now, requestID)
		redis.call('EXPIRE', key, ttl)
		return {redis.call('ZCARD', key), now}
	`)

	// acquireOpenAIWSIngressLeaseScript atomically reaps crashed members and
	// acquires or refreshes one API-key-scoped ingress lease using Redis TIME.
	acquireOpenAIWSIngressLeaseScript = redis.NewScript(`
		redis.replicate_commands()
		local key = KEYS[1]
		local maxConnections = tonumber(ARGV[1])
		local ttl = tonumber(ARGV[2])
		local leaseID = ARGV[3]
		local now = tonumber(redis.call('TIME')[1])
		local expireBefore = now - ttl
		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		if redis.call('ZSCORE', key, leaseID) ~= false then
			redis.call('ZADD', key, now, leaseID)
			redis.call('EXPIRE', key, ttl)
			return 1
		end
		if redis.call('ZCARD', key) < maxConnections then
			redis.call('ZADD', key, now, leaseID)
			redis.call('EXPIRE', key, ttl)
			return 1
		end
		return 0
	`)

	// refreshOpenAIWSIngressLeaseScript does not recreate a missing member: a
	// process that lost its lease must terminate its local WebSocket instead of
	// silently continuing beyond the distributed cap.
	refreshOpenAIWSIngressLeaseScript = redis.NewScript(`
		redis.replicate_commands()
		local key = KEYS[1]
		local ttl = tonumber(ARGV[1])
		local leaseID = ARGV[2]
		local now = tonumber(redis.call('TIME')[1])
		local expireBefore = now - ttl
		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		if redis.call('ZSCORE', key, leaseID) == false then
			return 0
		end
		redis.call('ZADD', key, now, leaseID)
		redis.call('EXPIRE', key, ttl)
		return 1
	`)

	// incrementWaitScript - refreshes TTL on each increment to keep queue depth accurate
	// KEYS[1] = wait queue key
	// ARGV[1] = maxWait
	// ARGV[2] = TTL in seconds
	// 返回 {是否成功, Redis 当前秒}，供 Go 侧免额外 TIME 往返写活跃索引。
	incrementWaitScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local current = redis.call('GET', KEYS[1])
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		local now = tonumber(redis.call('TIME')[1])

		if current >= tonumber(ARGV[1]) then
			return {0, now, current}
		end

		local next = redis.call('INCR', KEYS[1])

		-- Refresh TTL so long-running traffic doesn't expire active queue counters.
		redis.call('EXPIRE', KEYS[1], ARGV[2])

		return {1, now, next}
	`)

	// incrementAccountWaitScript - account-level wait queue count (refresh TTL on each increment)
	// 返回值同 incrementWaitScript：{是否成功, Redis 当前秒}。
	incrementAccountWaitScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local current = redis.call('GET', KEYS[1])
		if current == false then
			current = 0
		else
			current = tonumber(current)
		end
		local now = tonumber(redis.call('TIME')[1])

		if current >= tonumber(ARGV[1]) then
			return {0, now}
		end

		redis.call('INCR', KEYS[1])

		-- Refresh TTL so long-running traffic doesn't expire active queue counters.
		redis.call('EXPIRE', KEYS[1], ARGV[2])

		return {1, now}
	`)

	// decrementWaitScript - same as before
	decrementWaitScript = redis.NewScript(`
			redis.replicate_commands()
			local current = redis.call('GET', KEYS[1])
			local remaining = 0
			if current ~= false and tonumber(current) > 0 then
				remaining = redis.call('DECR', KEYS[1])
			end
			if remaining <= 0 then
				remaining = 0
				redis.call('DEL', KEYS[1])
			end
			local now = tonumber(redis.call('TIME')[1])
			return {remaining, now}
		`)

	// releaseSlotStateScript atomically releases a user slot and returns the
	// remaining count plus Redis time for realtime trend observation.
	releaseSlotStateScript = redis.NewScript(`
		redis.replicate_commands()
		local ttl = tonumber(ARGV[1])
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - ttl)
		redis.call('ZREM', KEYS[1], ARGV[2])
		local remaining = redis.call('ZCARD', KEYS[1])
		if remaining == 0 then
			redis.call('DEL', KEYS[1])
		end
		return {remaining, now}
	`)

	// cleanupExpiredSlotsScript 清理单个账号/用户有序集合中过期槽位
	// KEYS[1] = 有序集合键
	// ARGV[1] = TTL（秒）
	cleanupExpiredSlotsScript = redis.NewScript(`
		-- Redis 3.2-4.x compat: opt into effects replication so redis.call('TIME')
		-- replicates correctly. No-op on Redis 5.0+ (effects replication is default).
		redis.replicate_commands()
		local key = KEYS[1]
		local ttl = tonumber(ARGV[1])
		local timeResult = redis.call('TIME')
		local now = tonumber(timeResult[1])
		local expireBefore = now - ttl
		redis.call('ZREMRANGEBYSCORE', key, '-inf', expireBefore)
		if redis.call('ZCARD', key) == 0 then
			redis.call('DEL', key)
		else
			redis.call('EXPIRE', key, ttl)
		end
		return 1
	`)

	// startupCleanupSlotScript 清理单个槽位 key 中非当前进程前缀的成员，避免 Redis Cluster CROSSSLOT。
	// KEYS[1] 是有序集合键，ARGV[1] 是当前进程前缀，ARGV[2] 是槽位 TTL。
	// 返回 {清除数量, 剩余成员数}，Go 侧据剩余数决定索引 member 去留，无需再回读槽位。
	startupCleanupSlotScript = redis.NewScript(`
		local key = KEYS[1]
		local activePrefix = ARGV[1]
		local slotTTL = tonumber(ARGV[2])
		local removed = 0
		local members = redis.call('ZRANGE', key, 0, -1)
		for _, member in ipairs(members) do
			if string.sub(member, 1, string.len(activePrefix)) ~= activePrefix then
				removed = removed + redis.call('ZREM', key, member)
			end
		end
		local remaining = redis.call('ZCARD', key)
		if remaining == 0 then
			redis.call('DEL', key)
		else
			redis.call('EXPIRE', key, slotTTL)
		end
		return {removed, remaining}
	`)

	requestBodyLaneAcquireScript = redis.NewScript(`
		redis.replicate_commands()
		local scopeKey = KEYS[1]
		local userKey = KEYS[2]
		local waitKey = KEYS[3]
		local scopeWaitKey = KEYS[4]
		local accountScopeKey = KEYS[5]
		local maxPermits = tonumber(ARGV[1])
		local weight = tonumber(ARGV[2])
		local ttl = tonumber(ARGV[3])
		local requestID = ARGV[4]
		local lane = ARGV[5]
		local scopeWaitMember = ARGV[6]
		local accountMaxPermits = tonumber(ARGV[7])
		local activeMember = lane .. ':' .. requestID
		local pendingActiveMember = 'pending_active:' .. requestID
		local now = tonumber(redis.call('TIME')[1])
		local expireBefore = now - ttl

		redis.call('ZREMRANGEBYSCORE', scopeKey, '-inf', expireBefore)
		redis.call('ZREMRANGEBYSCORE', userKey, '-inf', expireBefore)
		if accountMaxPermits > 0 then
			redis.call('ZREMRANGEBYSCORE', accountScopeKey, '-inf', expireBefore)
		end
		if redis.call('ZSCORE', userKey, pendingActiveMember) ~= false then
			redis.call('ZADD', userKey, now, pendingActiveMember)
		end

		local function state(result)
			local heavyActive = 0
			local recoveryActive = 0
			local pendingActive = 0
			local pendingWaiting = 0
			local members = redis.call('ZRANGE', userKey, 0, -1)
			for _, member in ipairs(members) do
				if string.sub(member, 1, 6) == 'heavy:' then
					heavyActive = heavyActive + 1
				elseif string.sub(member, 1, 9) == 'recovery:' then
					recoveryActive = recoveryActive + 1
				elseif string.sub(member, 1, 15) == 'pending_active:' then
					pendingActive = pendingActive + 1
				elseif string.sub(member, 1, 16) == 'pending_waiting:' then
					pendingWaiting = pendingWaiting + 1
				end
			end
			local waitingLane = redis.call('GET', waitKey)
			local heavyWaiting = 0
			local recoveryWaiting = 0
			if waitingLane and string.sub(waitingLane, 1, 6) == 'heavy:' then
				heavyWaiting = 1
			elseif waitingLane and string.sub(waitingLane, 1, 9) == 'recovery:' then
				recoveryWaiting = 1
			end
			return {result, now, heavyActive, heavyWaiting, recoveryActive, recoveryWaiting, pendingActive, pendingWaiting}
		end

		if redis.call('ZSCORE', userKey, activeMember) ~= false then
			redis.call('ZADD', userKey, now, activeMember)
			for i = 1, weight do
				redis.call('ZADD', scopeKey, now, requestID .. ':' .. i)
				if accountMaxPermits > 0 then
					redis.call('ZADD', accountScopeKey, now, requestID .. ':' .. i)
				end
			end
			if redis.call('GET', waitKey) == activeMember then
				redis.call('DEL', waitKey)
				redis.call('ZREM', scopeWaitKey, scopeWaitMember)
			end
			redis.call('EXPIRE', userKey, ttl)
			redis.call('EXPIRE', scopeKey, ttl)
			if accountMaxPermits > 0 then redis.call('EXPIRE', accountScopeKey, ttl) end
			return state(1)
		end

		local activeLaneCount = 0
		local activeMembers = redis.call('ZRANGE', userKey, 0, -1)
		for _, member in ipairs(activeMembers) do
			if string.sub(member, 1, 6) == 'heavy:' or string.sub(member, 1, 9) == 'recovery:' then
				activeLaneCount = activeLaneCount + 1
			end
		end
		if activeLaneCount >= 1 then
			return state(0)
		end
		if redis.call('ZCARD', scopeKey) + weight > maxPermits then
			return state(0)
		end
		if accountMaxPermits > 0 and redis.call('ZCARD', accountScopeKey) + weight > accountMaxPermits then
			return state(0)
		end

		redis.call('ZADD', userKey, now, activeMember)
		for i = 1, weight do
			redis.call('ZADD', scopeKey, now, requestID .. ':' .. i)
			if accountMaxPermits > 0 then
				redis.call('ZADD', accountScopeKey, now, requestID .. ':' .. i)
			end
		end
		if redis.call('GET', waitKey) == activeMember then
			redis.call('DEL', waitKey)
			redis.call('ZREM', scopeWaitKey, scopeWaitMember)
		end
		redis.call('EXPIRE', userKey, ttl)
		redis.call('EXPIRE', scopeKey, ttl)
		if accountMaxPermits > 0 then redis.call('EXPIRE', accountScopeKey, ttl) end
		return state(1)
	`)

	requestBodyLaneReleaseScript = redis.NewScript(`
		redis.replicate_commands()
		local requestID = ARGV[1]
		local weight = tonumber(ARGV[2])
		local lane = ARGV[3]
		local ttl = tonumber(ARGV[4])
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', now - ttl)
		redis.call('ZREM', KEYS[2], lane .. ':' .. requestID)
		redis.call('ZREM', KEYS[2], requestID)
		for i = 1, weight do
			redis.call('ZREM', KEYS[1], requestID .. ':' .. i)
			redis.call('ZREM', KEYS[4], requestID .. ':' .. i)
		end
		local heavyActive = 0
		local recoveryActive = 0
		local pendingActive = 0
		local pendingWaiting = 0
		local members = redis.call('ZRANGE', KEYS[2], 0, -1)
		for _, member in ipairs(members) do
			if string.sub(member, 1, 6) == 'heavy:' then
				heavyActive = heavyActive + 1
			elseif string.sub(member, 1, 9) == 'recovery:' then
				recoveryActive = recoveryActive + 1
			elseif string.sub(member, 1, 15) == 'pending_active:' then
				pendingActive = pendingActive + 1
			elseif string.sub(member, 1, 16) == 'pending_waiting:' then
				pendingWaiting = pendingWaiting + 1
			end
		end
		local waitingLane = redis.call('GET', KEYS[3])
		local heavyWaiting = 0
		local recoveryWaiting = 0
		if waitingLane and string.sub(waitingLane, 1, 6) == 'heavy:' then
			heavyWaiting = 1
		elseif waitingLane and string.sub(waitingLane, 1, 9) == 'recovery:' then
			recoveryWaiting = 1
		end
		return {now, heavyActive, heavyWaiting, recoveryActive, recoveryWaiting, pendingActive, pendingWaiting}
	`)

	requestBodyLaneRefreshScript = redis.NewScript(`
		redis.replicate_commands()
		local lane = ARGV[1]
		local requestID = ARGV[2]
		local weight = tonumber(ARGV[3])
		local ttl = tonumber(ARGV[4])
		local hasAccountScope = tonumber(ARGV[5])
		local activeMember = lane .. ':' .. requestID
		local pendingActiveMember = 'pending_active:' .. requestID
		local now = tonumber(redis.call('TIME')[1])
		local expireBefore = now - ttl
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', expireBefore)
		redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', expireBefore)
		if hasAccountScope > 0 then redis.call('ZREMRANGEBYSCORE', KEYS[3], '-inf', expireBefore) end
		if redis.call('ZSCORE', KEYS[2], activeMember) == false then return 0 end
		for i = 1, weight do
			if redis.call('ZSCORE', KEYS[1], requestID .. ':' .. i) == false then return 0 end
			if hasAccountScope > 0 and redis.call('ZSCORE', KEYS[3], requestID .. ':' .. i) == false then return 0 end
		end
		redis.call('ZADD', KEYS[2], now, activeMember)
		if redis.call('ZSCORE', KEYS[2], pendingActiveMember) ~= false then
			redis.call('ZADD', KEYS[2], now, pendingActiveMember)
		end
		for i = 1, weight do
			redis.call('ZADD', KEYS[1], now, requestID .. ':' .. i)
			if hasAccountScope > 0 then redis.call('ZADD', KEYS[3], now, requestID .. ':' .. i) end
		end
		redis.call('EXPIRE', KEYS[1], ttl)
		redis.call('EXPIRE', KEYS[2], ttl)
		if hasAccountScope > 0 then redis.call('EXPIRE', KEYS[3], ttl) end
		return 1
	`)

	requestBodyLaneIncrementWaitScript = redis.NewScript(`
		redis.replicate_commands()
		local userKey = KEYS[1]
		local waitKey = KEYS[2]
		local scopeWaitKey = KEYS[3]
		local lane = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local waiterID = ARGV[4]
		local maxScopeWait = tonumber(ARGV[5])
		local scopeWaitMember = ARGV[6]
		local waitValue = lane .. ':' .. waiterID
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', userKey, '-inf', now - tonumber(ARGV[3]))
		redis.call('ZREMRANGEBYSCORE', scopeWaitKey, '-inf', now - ttl)
		local allowed = 0
		local currentWaitValue = redis.call('GET', waitKey)
		local currentScopeWait = redis.call('ZSCORE', scopeWaitKey, scopeWaitMember)
		if currentWaitValue == false and (currentScopeWait ~= false or redis.call('ZCARD', scopeWaitKey) < maxScopeWait) then
			redis.call('SET', waitKey, waitValue, 'EX', ttl)
			redis.call('ZADD', scopeWaitKey, now, scopeWaitMember)
			redis.call('EXPIRE', scopeWaitKey, ttl)
			allowed = 1
		elseif currentWaitValue == waitValue then
			redis.call('EXPIRE', waitKey, ttl)
			redis.call('ZADD', scopeWaitKey, now, scopeWaitMember)
			redis.call('EXPIRE', scopeWaitKey, ttl)
			allowed = 1
		end
		local heavyActive = 0
		local recoveryActive = 0
		local pendingActive = 0
		local pendingWaiting = 0
		local members = redis.call('ZRANGE', userKey, 0, -1)
		for _, member in ipairs(members) do
			if string.sub(member, 1, 6) == 'heavy:' then
				heavyActive = heavyActive + 1
			elseif string.sub(member, 1, 9) == 'recovery:' then
				recoveryActive = recoveryActive + 1
			elseif string.sub(member, 1, 15) == 'pending_active:' then
				pendingActive = pendingActive + 1
			elseif string.sub(member, 1, 16) == 'pending_waiting:' then
				pendingWaiting = pendingWaiting + 1
			end
		end
		local waitingLane = redis.call('GET', waitKey)
		local heavyWaiting = 0
		local recoveryWaiting = 0
		if waitingLane and string.sub(waitingLane, 1, 6) == 'heavy:' then
			heavyWaiting = 1
		elseif waitingLane and string.sub(waitingLane, 1, 9) == 'recovery:' then
			recoveryWaiting = 1
		end
		return {allowed, now, heavyActive, heavyWaiting, recoveryActive, recoveryWaiting, pendingActive, pendingWaiting}
	`)

	requestBodyLaneDecrementWaitScript = redis.NewScript(`
		redis.replicate_commands()
		local userKey = KEYS[1]
		local waitKey = KEYS[2]
		local scopeWaitKey = KEYS[3]
		local lane = ARGV[1]
		local ttl = tonumber(ARGV[2])
		local waitValue = lane .. ':' .. ARGV[3]
		local scopeWaitMember = ARGV[4]
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', userKey, '-inf', now - ttl)
		if redis.call('GET', waitKey) == waitValue then
			redis.call('DEL', waitKey)
		end
		redis.call('ZREM', scopeWaitKey, scopeWaitMember)
		local heavyActive = 0
		local recoveryActive = 0
		local pendingActive = 0
		local pendingWaiting = 0
		local members = redis.call('ZRANGE', userKey, 0, -1)
		for _, member in ipairs(members) do
			if string.sub(member, 1, 6) == 'heavy:' then
				heavyActive = heavyActive + 1
			elseif string.sub(member, 1, 9) == 'recovery:' then
				recoveryActive = recoveryActive + 1
			elseif string.sub(member, 1, 15) == 'pending_active:' then
				pendingActive = pendingActive + 1
			elseif string.sub(member, 1, 16) == 'pending_waiting:' then
				pendingWaiting = pendingWaiting + 1
			end
		end
		local waitingLane = redis.call('GET', waitKey)
		local heavyWaiting = 0
		local recoveryWaiting = 0
		if waitingLane and string.sub(waitingLane, 1, 6) == 'heavy:' then
			heavyWaiting = 1
		elseif waitingLane and string.sub(waitingLane, 1, 9) == 'recovery:' then
			recoveryWaiting = 1
		end
		return {now, heavyActive, heavyWaiting, recoveryActive, recoveryWaiting, pendingActive, pendingWaiting}
	`)

	requestBodyClassificationStateScript = redis.NewScript(`
		redis.replicate_commands()
		local userKey = KEYS[1]
		local waitKey = KEYS[2]
		local requestID = ARGV[1]
		local active = ARGV[2] == '1'
		local waiting = ARGV[3] == '1'
		local ttl = tonumber(ARGV[4])
		local now = tonumber(redis.call('TIME')[1])
		redis.call('ZREMRANGEBYSCORE', userKey, '-inf', now - ttl)

		local activeMember = 'pending_active:' .. requestID
		local waitingMember = 'pending_waiting:' .. requestID
		if active then
			redis.call('ZADD', userKey, now, activeMember)
		else
			redis.call('ZREM', userKey, activeMember)
		end
		if waiting then
			redis.call('ZADD', userKey, now, waitingMember)
		else
			redis.call('ZREM', userKey, waitingMember)
		end

		local heavyActive = 0
		local recoveryActive = 0
		local pendingActive = 0
		local pendingWaiting = 0
		local members = redis.call('ZRANGE', userKey, 0, -1)
		for _, member in ipairs(members) do
			if string.sub(member, 1, 6) == 'heavy:' then
				heavyActive = heavyActive + 1
			elseif string.sub(member, 1, 9) == 'recovery:' then
				recoveryActive = recoveryActive + 1
			elseif string.sub(member, 1, 15) == 'pending_active:' then
				pendingActive = pendingActive + 1
			elseif string.sub(member, 1, 16) == 'pending_waiting:' then
				pendingWaiting = pendingWaiting + 1
			end
		end
		local waitingLane = redis.call('GET', waitKey)
		local heavyWaiting = 0
		local recoveryWaiting = 0
		if waitingLane and string.sub(waitingLane, 1, 6) == 'heavy:' then
			heavyWaiting = 1
		elseif waitingLane and string.sub(waitingLane, 1, 9) == 'recovery:' then
			recoveryWaiting = 1
		end
		if redis.call('ZCARD', userKey) > 0 then
			redis.call('EXPIRE', userKey, ttl)
		end
		return {now, heavyActive, heavyWaiting, recoveryActive, recoveryWaiting, pendingActive, pendingWaiting}
	`)
)

type concurrencyCache struct {
	rdb                 *redis.Client
	slotTTLSeconds      int // 槽位过期时间（秒）
	waitQueueTTLSeconds int // 等待队列过期时间（秒）
}

var _ service.RequestBodyAdmissionCache = (*concurrencyCache)(nil)
var _ service.RequestBodyAdmissionStateCache = (*concurrencyCache)(nil)
var _ service.RequestBodyAdmissionScopedWaitStateCache = (*concurrencyCache)(nil)
var _ service.RequestBodyAdmissionLeaseCache = (*concurrencyCache)(nil)

// NewConcurrencyCache 创建并发控制缓存
// slotTTLMinutes: 槽位过期时间（分钟），0 或负数使用默认值 15 分钟
// waitQueueTTLSeconds: 等待队列过期时间（秒），0 或负数使用 slot TTL
func NewConcurrencyCache(rdb *redis.Client, slotTTLMinutes int, waitQueueTTLSeconds int) service.ConcurrencyCache {
	if slotTTLMinutes <= 0 {
		slotTTLMinutes = defaultSlotTTLMinutes
	}
	if waitQueueTTLSeconds <= 0 {
		waitQueueTTLSeconds = slotTTLMinutes * 60
	}
	return &concurrencyCache{
		rdb:                 rdb,
		slotTTLSeconds:      slotTTLMinutes * 60,
		waitQueueTTLSeconds: waitQueueTTLSeconds,
	}
}

// Helper functions for key generation
func accountSlotKey(accountID int64) string {
	return fmt.Sprintf("%s%d", accountSlotKeyPrefix, accountID)
}

func userSlotKey(userID int64) string {
	return fmt.Sprintf("%s%d", userSlotKeyPrefix, userID)
}

func apiKeySlotKey(apiKeyID int64) string {
	return fmt.Sprintf("%s%d", apiKeySlotKeyPrefix, apiKeyID)
}

func liveAccountSlotKey(accountID int64) string {
	return fmt.Sprintf("%s%d", liveAccountSlotKeyPrefix, accountID)
}

func liveUserSlotKey(userID int64) string {
	return fmt.Sprintf("%s%d", liveUserSlotKeyPrefix, userID)
}

func liveAPIKeySlotKey(apiKeyID int64) string {
	return fmt.Sprintf("%s%d", liveAPIKeySlotKeyPrefix, apiKeyID)
}

func openAIWSIngressLeaseKey(apiKeyID int64) string {
	return fmt.Sprintf("%s%d", openAIWSIngressLeaseKeyPrefix, apiKeyID)
}

func waitQueueKey(userID int64) string {
	return fmt.Sprintf("%s%d", waitQueueKeyPrefix, userID)
}

func accountWaitKey(accountID int64) string {
	return fmt.Sprintf("%s%d", accountWaitKeyPrefix, accountID)
}

func requestBodyLaneScopeKey(lane service.RequestBodyLane, scopeID int64) string {
	return fmt.Sprintf("%s{admission}:%s:scope:%d", requestBodyLaneKeyPrefix, lane, scopeID)
}

func requestBodyLaneScopeKeys(lane service.RequestBodyLane, accountID int64) (primary, account string, accountLimit int) {
	if lane == service.RequestBodyLaneRecovery {
		primary = requestBodyLaneScopeKey(lane, 0)
		if accountID > 0 {
			account = requestBodyLaneScopeKey(lane, accountID)
			accountLimit = service.RequestBodyRecoveryAccountConcurrency
		}
		return primary, account, accountLimit
	}
	primary = requestBodyLaneScopeKey(lane, accountID)
	return primary, primary, 0
}

func requestBodyLaneWaitScopeID(lane service.RequestBodyLane, scopeID int64) int64 {
	if lane == service.RequestBodyLaneRecovery {
		return 0
	}
	return scopeID
}

func requestBodyLaneUserKey(userID int64) string {
	return fmt.Sprintf("%s{admission}:user:%d", requestBodyLaneKeyPrefix, userID)
}

func requestBodyLaneWaitKey(userID int64) string {
	return fmt.Sprintf("%s{admission}:user:%d", requestBodyLaneWaitKeyPrefix, userID)
}

func requestBodyLaneScopeWaitKey(lane service.RequestBodyLane, scopeID int64) string {
	return fmt.Sprintf("%s{admission}:%s:scope:%d", requestBodyLaneWaitKeyPrefix, lane, scopeID)
}

func requestBodyLaneScopeWaitMember(userID int64, waiterID string) string {
	return fmt.Sprintf("%d:%s", userID, waiterID)
}

// redisUnixSeconds 统一使用 Redis 服务器时间，避免多实例本地时钟漂移导致索引提前/延后过期。
func (c *concurrencyCache) redisUnixSeconds(ctx context.Context) (int64, error) {
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return 0, fmt.Errorf("redis TIME: %w", err)
	}
	return now.Unix(), nil
}

// slotIndexSpec 描述一个活跃索引及其对应的槽位/等待键构造方式。
// 用具名字段避免把 slotKey/waitKey 两个同签名函数按位置传参时写反。
type slotIndexSpec struct {
	indexKey string
	slotKey  func(int64) string
	waitKey  func(int64) string
}

var (
	accountSlotIndex = slotIndexSpec{indexKey: accountActiveIndexKey, slotKey: accountSlotKey, waitKey: accountWaitKey}
	userSlotIndex    = slotIndexSpec{indexKey: userActiveIndexKey, slotKey: userSlotKey, waitKey: waitQueueKey}
)

// touchActiveIndexAt 是写路径上的轻量标记：主操作已成功时，尽力把 ID 放入活跃索引，
// score 为给定的绝对过期时间（Redis Unix 秒）。索引失败不影响并发槽位/等待队列本身，
// 后续释放或清理会再次校正，因此只记日志不上抛。
func (c *concurrencyCache) touchActiveIndexAt(ctx context.Context, indexKey string, id int64, expireAt int64) {
	if c == nil || c.rdb == nil || id <= 0 || expireAt <= 0 {
		return
	}
	if err := c.rdb.ZAdd(ctx, indexKey, redis.Z{
		Score:  float64(expireAt),
		Member: strconv.FormatInt(id, 10),
	}).Err(); err != nil {
		logger.LegacyPrintf("repository.concurrency", "Warning: touch active index %s for %d failed: %v", indexKey, id, err)
	}
}

func (c *concurrencyCache) refreshAccountActiveIndex(ctx context.Context, accountID int64) {
	c.refreshActiveIndex(ctx, accountActiveIndexKey, accountID, accountSlotKey(accountID), accountWaitKey(accountID))
}

func (c *concurrencyCache) refreshUserActiveIndex(ctx context.Context, userID int64) {
	c.refreshActiveIndex(ctx, userActiveIndexKey, userID, userSlotKey(userID), waitQueueKey(userID))
}

// refreshActiveIndex 以 Redis 中的真实槽位/等待数为准重建索引状态。
// 释放槽位、等待计数减少、清理过期成员后都会调用它，防止索引残留。
// 索引维护是 best-effort：失败只记日志，不影响主流程。
func (c *concurrencyCache) refreshActiveIndex(ctx context.Context, indexKey string, id int64, slotKey, waitKey string) {
	if c == nil || c.rdb == nil || id <= 0 {
		return
	}
	now, err := c.redisUnixSeconds(ctx)
	if err != nil {
		logger.LegacyPrintf("repository.concurrency", "Warning: refresh active index %s for %d failed: %v", indexKey, id, err)
		return
	}

	load, err := c.readActiveLoadForKey(ctx, id, slotKey, waitKey, now)
	if err != nil {
		logger.LegacyPrintf("repository.concurrency", "Warning: refresh active index %s for %d failed: %v", indexKey, id, err)
		return
	}
	member := strconv.FormatInt(id, 10)
	if load.slotCount == 0 && load.waitCount <= 0 {
		if err := c.rdb.ZRem(ctx, indexKey, member).Err(); err != nil {
			logger.LegacyPrintf("repository.concurrency", "Warning: remove active index member %s from %s failed: %v", member, indexKey, err)
		}
		return
	}

	ttlSeconds := c.activeIndexTTL(load.slotCount, load.waitCount)
	if ttlSeconds <= 0 {
		return
	}
	c.touchActiveIndexAt(ctx, indexKey, id, now+int64(ttlSeconds))
}

type activeIndexLoad struct {
	id        int64
	member    string
	slotCount int
	waitCount int
}

// activeIndexTTL 取槽位 TTL 与等待队列 TTL 中仍然需要关注的较大值。
// 只要并发槽位或等待计数还有负载，就保留索引；两者都为 0 时调用方会删除索引。
func (c *concurrencyCache) activeIndexTTL(slotCount int, waitCount int) int {
	ttlSeconds := 0
	if slotCount > 0 {
		ttlSeconds = c.slotTTLSeconds
	}
	if waitCount > 0 && c.waitQueueTTLSeconds > ttlSeconds {
		ttlSeconds = c.waitQueueTTLSeconds
	}
	return ttlSeconds
}

// readActiveLoadForKey 读取单个 ID 的当前负载，并顺手清理该槽位集合中的过期成员。
func (c *concurrencyCache) readActiveLoadForKey(ctx context.Context, id int64, slotKey, waitKey string, now int64) (activeIndexLoad, error) {
	cutoffTime := now - int64(c.slotTTLSeconds)
	pipe := c.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
	zcardCmd := pipe.ZCard(ctx, slotKey)
	getCmd := pipe.Get(ctx, waitKey)
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return activeIndexLoad{}, fmt.Errorf("pipeline exec: %w", err)
	}

	waitCount := 0
	if v, err := getCmd.Int(); err == nil && v > 0 {
		waitCount = v
	}
	return activeIndexLoad{
		id:        id,
		member:    strconv.FormatInt(id, 10),
		slotCount: int(zcardCmd.Val()),
		waitCount: waitCount,
	}, nil
}

// readIndexLoads 批量读取索引候选的真实负载（账号/用户通用）。
// 分块 Pipeline 可以减少 Redis 往返，同时避免一次 Pipeline 塞入过多命令。
func (c *concurrencyCache) readIndexLoads(ctx context.Context, spec slotIndexSpec, members []string, now int64) ([]activeIndexLoad, []string, error) {
	loads := make([]activeIndexLoad, 0, len(members))
	staleMembers := make([]string, 0)
	candidates := make([]activeIndexLoad, 0, len(members))
	for _, member := range members {
		id, err := strconv.ParseInt(member, 10, 64)
		if err != nil || id <= 0 {
			staleMembers = append(staleMembers, member)
			continue
		}
		candidates = append(candidates, activeIndexLoad{id: id, member: member})
	}

	cutoffTime := now - int64(c.slotTTLSeconds)
	for start := 0; start < len(candidates); start += activeIndexPipelineChunkSize {
		end := start + activeIndexPipelineChunkSize
		if end > len(candidates) {
			end = len(candidates)
		}
		chunk := candidates[start:end]

		pipe := c.rdb.Pipeline()
		type loadCmd struct {
			activeIndexLoad
			zcardCmd *redis.IntCmd
			getCmd   *redis.StringCmd
		}
		cmds := make([]loadCmd, 0, len(chunk))
		for _, candidate := range chunk {
			slotKey := spec.slotKey(candidate.id)
			waitKey := spec.waitKey(candidate.id)
			pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
			cmds = append(cmds, loadCmd{
				activeIndexLoad: candidate,
				zcardCmd:        pipe.ZCard(ctx, slotKey),
				getCmd:          pipe.Get(ctx, waitKey),
			})
		}
		if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
			return nil, nil, fmt.Errorf("pipeline exec: %w", err)
		}
		for _, cmd := range cmds {
			waitCount := 0
			if v, err := cmd.getCmd.Int(); err == nil && v > 0 {
				waitCount = v
			}
			loads = append(loads, activeIndexLoad{
				id:        cmd.id,
				member:    cmd.member,
				slotCount: int(cmd.zcardCmd.Val()),
				waitCount: waitCount,
			})
		}
	}

	return loads, staleMembers, nil
}

// removeActiveIndexMembers 清理无效 member；这是辅助索引的维护动作，调用方无需因为失败中断主流程。
func (c *concurrencyCache) removeActiveIndexMembers(ctx context.Context, indexKey string, members []string) {
	if len(members) == 0 {
		return
	}
	args := make([]any, 0, len(members))
	for _, member := range members {
		args = append(args, member)
	}
	if err := c.rdb.ZRem(ctx, indexKey, args...).Err(); err != nil {
		logger.LegacyPrintf("repository.concurrency", "Warning: remove %d active index members from %s failed: %v", len(members), indexKey, err)
	}
}

// runScriptInt64Pair 执行返回两元素整数数组的 Lua 脚本并解析（如 {result, now}、{removed, remaining}）。
func runScriptInt64Pair(ctx context.Context, rdb *redis.Client, script *redis.Script, keys []string, args ...any) (int64, int64, error) {
	raw, err := script.Run(ctx, rdb, keys, args...).Result()
	if err != nil {
		return 0, 0, err
	}
	first, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("parse script value 0: %w", err)
	}
	second, err := redisScriptInt64At(raw, 1)
	if err != nil {
		return 0, 0, fmt.Errorf("parse script value 1: %w", err)
	}
	return first, second, nil
}

// Account slot operations

func (c *concurrencyCache) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error) {
	key := accountSlotKey(accountID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取，确保多实例时钟一致
	result, now, err := runScriptInt64Pair(ctx, c.rdb, acquireScript, []string{key, liveAccountSlotKey(accountID)}, maxConcurrency, c.slotTTLSeconds, requestID)
	if err != nil {
		return false, err
	}
	if result == 1 {
		// 成功占槽后标记活跃账号，后台清理即可从索引定位候选账号。
		c.touchActiveIndexAt(ctx, accountActiveIndexKey, accountID, now+int64(c.slotTTLSeconds))
	}
	return result == 1, nil
}

func (c *concurrencyCache) ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error {
	key := accountSlotKey(accountID)
	if err := c.rdb.ZRem(ctx, key, requestID).Err(); err != nil {
		return err
	}
	// 释放后用真实负载刷新索引；若没有槽位和等待计数，会移除索引 member。
	c.refreshAccountActiveIndex(ctx, accountID)
	return nil
}

func (c *concurrencyCache) AcquireRequestBodyLane(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	maxPermits, weight int,
	requestID string,
) (bool, error) {
	acquired, _, _, err := c.AcquireRequestBodyLaneWithState(ctx, lane, scopeID, userID, maxPermits, weight, requestID)
	return acquired, err
}

func (c *concurrencyCache) AcquireRequestBodyLaneWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	maxPermits, weight int,
	requestID string,
) (bool, service.RequestBodyLaneUserLoad, time.Time, error) {
	if maxPermits <= 0 || weight <= 0 || requestID == "" {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, nil
	}
	primaryScopeKey, accountScopeKey, accountLimit := requestBodyLaneScopeKeys(lane, scopeID)
	if accountScopeKey == "" {
		accountScopeKey = primaryScopeKey
	}
	raw, err := requestBodyLaneAcquireScript.Run(
		ctx,
		c.rdb,
		[]string{
			primaryScopeKey,
			requestBodyLaneUserKey(userID),
			requestBodyLaneWaitKey(userID),
			requestBodyLaneScopeWaitKey(lane, requestBodyLaneWaitScopeID(lane, scopeID)),
			accountScopeKey,
		},
		maxPermits,
		weight,
		requestBodyAdmissionLeaseTTLSeconds,
		requestID,
		string(lane),
		requestBodyLaneScopeWaitMember(userID, requestID),
		accountLimit,
	).Result()
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	result, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body acquire result: %w", err)
	}
	now, err := redisScriptInt64At(raw, 1)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body acquire time: %w", err)
	}
	state, err := requestBodyLaneUserLoadAt(raw, 2)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	c.refreshRequestBodyActiveIndexAt(ctx, userID, state, now)
	return result == 1, state, time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) ReleaseRequestBodyLane(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	weight int,
	requestID string,
) error {
	_, _, err := c.ReleaseRequestBodyLaneWithState(ctx, lane, scopeID, userID, weight, requestID)
	return err
}

func (c *concurrencyCache) ReleaseRequestBodyLaneWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	weight int,
	requestID string,
) (service.RequestBodyLaneUserLoad, time.Time, error) {
	if weight <= 0 {
		weight = 1
	}
	primaryScopeKey, accountScopeKey, _ := requestBodyLaneScopeKeys(lane, scopeID)
	if accountScopeKey == "" {
		accountScopeKey = primaryScopeKey
	}
	raw, err := requestBodyLaneReleaseScript.Run(
		ctx,
		c.rdb,
		[]string{primaryScopeKey, requestBodyLaneUserKey(userID), requestBodyLaneWaitKey(userID), accountScopeKey},
		requestID,
		weight,
		string(lane),
		requestBodyAdmissionLeaseTTLSeconds,
	).Result()
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	now, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body release time: %w", err)
	}
	state, err := requestBodyLaneUserLoadAt(raw, 1)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	c.refreshRequestBodyActiveIndexAt(ctx, userID, state, now)
	return state, time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) RefreshRequestBodyLane(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	weight int,
	requestID string,
) (bool, error) {
	if c == nil || c.rdb == nil || requestID == "" {
		return false, nil
	}
	if weight <= 0 {
		weight = 1
	}
	primaryScopeKey, accountScopeKey, accountLimit := requestBodyLaneScopeKeys(lane, scopeID)
	if accountScopeKey == "" {
		accountScopeKey = primaryScopeKey
	}
	result, err := requestBodyLaneRefreshScript.Run(
		ctx,
		c.rdb,
		[]string{primaryScopeKey, requestBodyLaneUserKey(userID), accountScopeKey},
		string(lane), requestID, weight, requestBodyAdmissionLeaseTTLSeconds, accountLimit,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) IncrementRequestBodyLaneWaitCount(ctx context.Context, userID int64, maxWait int, waiterID string) (bool, error) {
	allowed, _, _, err := c.IncrementRequestBodyLaneWaitCountWithState(ctx, service.RequestBodyLaneHeavy, userID, maxWait, waiterID)
	return allowed, err
}

func (c *concurrencyCache) IncrementRequestBodyLaneWaitCountWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	userID int64,
	maxWait int,
	waiterID string,
) (bool, service.RequestBodyLaneUserLoad, time.Time, error) {
	return c.IncrementRequestBodyLaneScopedWaitCountWithState(ctx, lane, userID, userID, maxWait, waiterID)
}

func (c *concurrencyCache) IncrementRequestBodyLaneScopedWaitCountWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	maxScopeWait int,
	waiterID string,
) (bool, service.RequestBodyLaneUserLoad, time.Time, error) {
	if maxScopeWait <= 0 || waiterID == "" {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, nil
	}
	waitScopeID := requestBodyLaneWaitScopeID(lane, scopeID)
	raw, err := requestBodyLaneIncrementWaitScript.Run(
		ctx,
		c.rdb,
		[]string{
			requestBodyLaneUserKey(userID),
			requestBodyLaneWaitKey(userID),
			requestBodyLaneScopeWaitKey(lane, waitScopeID),
		},
		string(lane),
		c.waitQueueTTLSeconds,
		requestBodyAdmissionLeaseTTLSeconds,
		waiterID,
		maxScopeWait,
		requestBodyLaneScopeWaitMember(userID, waiterID),
	).Result()
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	result, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body wait result: %w", err)
	}
	now, err := redisScriptInt64At(raw, 1)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body wait time: %w", err)
	}
	state, err := requestBodyLaneUserLoadAt(raw, 2)
	if err != nil {
		return false, service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	c.refreshRequestBodyActiveIndexAt(ctx, userID, state, now)
	return result == 1, state, time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) DecrementRequestBodyLaneWaitCount(ctx context.Context, userID int64, waiterID string) error {
	_, _, err := c.DecrementRequestBodyLaneWaitCountWithState(ctx, service.RequestBodyLaneHeavy, userID, waiterID)
	return err
}

func (c *concurrencyCache) DecrementRequestBodyLaneWaitCountWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	userID int64,
	waiterID string,
) (service.RequestBodyLaneUserLoad, time.Time, error) {
	return c.DecrementRequestBodyLaneScopedWaitCountWithState(ctx, lane, userID, userID, waiterID)
}

func (c *concurrencyCache) DecrementRequestBodyLaneScopedWaitCountWithState(
	ctx context.Context,
	lane service.RequestBodyLane,
	scopeID, userID int64,
	waiterID string,
) (service.RequestBodyLaneUserLoad, time.Time, error) {
	waitScopeID := requestBodyLaneWaitScopeID(lane, scopeID)
	raw, err := requestBodyLaneDecrementWaitScript.Run(
		ctx,
		c.rdb,
		[]string{
			requestBodyLaneUserKey(userID),
			requestBodyLaneWaitKey(userID),
			requestBodyLaneScopeWaitKey(lane, waitScopeID),
		},
		string(lane),
		requestBodyAdmissionLeaseTTLSeconds,
		waiterID,
		requestBodyLaneScopeWaitMember(userID, waiterID),
	).Result()
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	now, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body decrement time: %w", err)
	}
	state, err := requestBodyLaneUserLoadAt(raw, 1)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	c.refreshRequestBodyActiveIndexAt(ctx, userID, state, now)
	return state, time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) SetRequestBodyClassificationStateWithState(
	ctx context.Context,
	userID int64,
	requestID string,
	active, waiting bool,
) (service.RequestBodyLaneUserLoad, time.Time, error) {
	if requestID == "" {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, errors.New("request body classification request ID is required")
	}
	raw, err := requestBodyClassificationStateScript.Run(
		ctx,
		c.rdb,
		[]string{requestBodyLaneUserKey(userID), requestBodyLaneWaitKey(userID)},
		requestID,
		boolToRedisInt(active),
		boolToRedisInt(waiting),
		c.slotTTLSeconds,
	).Result()
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	now, err := redisScriptInt64At(raw, 0)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, fmt.Errorf("parse request body classification time: %w", err)
	}
	state, err := requestBodyLaneUserLoadAt(raw, 1)
	if err != nil {
		return service.RequestBodyLaneUserLoad{}, time.Time{}, err
	}
	c.refreshRequestBodyActiveIndexAt(ctx, userID, state, now)
	return state, time.Unix(now, 0).UTC(), nil
}

func boolToRedisInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func requestBodyLaneUserLoadAt(raw any, offset int) (service.RequestBodyLaneUserLoad, error) {
	values := make([]int64, 6)
	for index := range values {
		value, err := redisScriptInt64At(raw, offset+index)
		if err != nil {
			return service.RequestBodyLaneUserLoad{}, fmt.Errorf("parse request body lane state value %d: %w", index, err)
		}
		values[index] = value
	}
	return service.RequestBodyLaneUserLoad{
		HeavyActive:     int(values[0]),
		HeavyWaiting:    int(values[1]),
		RecoveryActive:  int(values[2]),
		RecoveryWaiting: int(values[3]),
		PendingActive:   int(values[4]),
		PendingWaiting:  int(values[5]),
	}, nil
}

func requestBodyLaneUserLoadActive(state service.RequestBodyLaneUserLoad) bool {
	return state.HeavyActive > 0 || state.HeavyWaiting > 0 || state.RecoveryActive > 0 || state.RecoveryWaiting > 0 ||
		state.PendingActive > 0 || state.PendingWaiting > 0
}

func (c *concurrencyCache) refreshRequestBodyActiveIndexAt(ctx context.Context, userID int64, state service.RequestBodyLaneUserLoad, now int64) {
	member := strconv.FormatInt(userID, 10)
	if !requestBodyLaneUserLoadActive(state) {
		c.removeActiveIndexMembers(ctx, requestBodyActiveIndexKey, []string{member})
		return
	}
	ttl := requestBodyAdmissionLeaseTTLSeconds
	if (state.HeavyWaiting > 0 || state.RecoveryWaiting > 0) && c.waitQueueTTLSeconds > ttl {
		ttl = c.waitQueueTTLSeconds
	}
	c.touchActiveIndexAt(ctx, requestBodyActiveIndexKey, userID, now+int64(ttl))
}

func (c *concurrencyCache) GetAccountConcurrency(ctx context.Context, accountID int64) (int, error) {
	key := accountSlotKey(accountID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取
	result, err := getCountScript.Run(ctx, c.rdb, []string{key, liveAccountSlotKey(accountID)}, c.slotTTLSeconds).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (c *concurrencyCache) GetAccountConcurrencyBatch(ctx context.Context, accountIDs []int64) (map[int64]int, error) {
	if len(accountIDs) == 0 {
		return map[int64]int{}, nil
	}

	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("redis TIME: %w", err)
	}
	cutoffTime := now.Unix() - int64(c.slotTTLSeconds)

	pipe := c.rdb.Pipeline()
	type accountCmd struct {
		accountID int64
		zcardCmd  *redis.IntCmd
		liveCmd   *redis.IntCmd
	}
	cmds := make([]accountCmd, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		slotKey := accountSlotKeyPrefix + strconv.FormatInt(accountID, 10)
		liveKey := liveAccountSlotKeyPrefix + strconv.FormatInt(accountID, 10)
		pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
		pipe.ZRemRangeByScore(ctx, liveKey, "-inf", strconv.FormatInt(now.Unix()-liveLeaseTTLSeconds, 10))
		cmds = append(cmds, accountCmd{
			accountID: accountID,
			zcardCmd:  pipe.ZCard(ctx, slotKey),
			liveCmd:   pipe.ZCard(ctx, liveKey),
		})
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}

	result := make(map[int64]int, len(accountIDs))
	for _, cmd := range cmds {
		result[cmd.accountID] = int(cmd.zcardCmd.Val() + cmd.liveCmd.Val())
	}
	return result, nil
}

// User slot operations

func (c *concurrencyCache) AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
	acquired, _, _, err := c.AcquireUserSlotWithState(ctx, userID, maxConcurrency, requestID)
	return acquired, err
}

func (c *concurrencyCache) AcquireUserSlotWithState(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, int, time.Time, error) {
	key := userSlotKey(userID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取，确保多实例时钟一致。
	// Live 会话与普通请求共享用户并发上限，返回值同时用于并发趋势打点。
	result, now, current, err := runScriptInt64Triple(ctx, c.rdb, acquireScript, []string{key, liveUserSlotKey(userID)}, maxConcurrency, c.slotTTLSeconds, requestID)
	if err != nil {
		return false, 0, time.Time{}, err
	}
	if result == 1 {
		// 成功占槽后标记活跃用户，避免启动清理依赖全量 SCAN。
		c.touchActiveIndexAt(ctx, userActiveIndexKey, userID, now+int64(c.slotTTLSeconds))
	}
	return result == 1, int(current), time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) TrackUserSlotWithState(ctx context.Context, userID int64, requestID string) (int, time.Time, error) {
	key := userSlotKey(userID)
	current, now, err := runScriptInt64Pair(ctx, c.rdb, trackUserSlotStateScript, []string{key}, c.slotTTLSeconds, requestID)
	if err != nil {
		return 0, time.Time{}, err
	}
	c.touchActiveIndexAt(ctx, userActiveIndexKey, userID, now+int64(c.slotTTLSeconds))
	return int(current), time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) ReleaseUserSlot(ctx context.Context, userID int64, requestID string) error {
	_, _, err := c.ReleaseUserSlotWithState(ctx, userID, requestID)
	return err
}

func (c *concurrencyCache) ReleaseUserSlotWithState(ctx context.Context, userID int64, requestID string) (int, time.Time, error) {
	key := userSlotKey(userID)
	remaining, now, err := runScriptInt64Pair(ctx, c.rdb, releaseSlotStateScript, []string{key}, c.slotTTLSeconds, requestID)
	if err != nil {
		return 0, time.Time{}, err
	}
	// 释放后按 Redis 中剩余负载修正索引状态。
	c.refreshUserActiveIndex(ctx, userID)
	return int(remaining), time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) GetUserConcurrency(ctx context.Context, userID int64) (int, error) {
	key := userSlotKey(userID)
	// 时间戳在 Lua 脚本内使用 Redis TIME 命令获取
	result, err := getCountScript.Run(ctx, c.rdb, []string{key, liveUserSlotKey(userID)}, c.slotTTLSeconds).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (c *concurrencyCache) TrackAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error {
	key := apiKeySlotKey(apiKeyID)
	_, err := trackSlotScript.Run(ctx, c.rdb, []string{key}, c.slotTTLSeconds, requestID).Result()
	return err
}

func (c *concurrencyCache) ReleaseAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error {
	key := apiKeySlotKey(apiKeyID)
	return c.rdb.ZRem(ctx, key, requestID).Err()
}

func (c *concurrencyCache) AcquireOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, maxConnections int, leaseID string) (bool, error) {
	if c == nil || c.rdb == nil || apiKeyID <= 0 || maxConnections <= 0 || leaseID == "" {
		return false, nil
	}
	result, err := acquireOpenAIWSIngressLeaseScript.Run(
		ctx,
		c.rdb,
		[]string{openAIWSIngressLeaseKey(apiKeyID)},
		maxConnections,
		openAIWSIngressLeaseTTLSeconds,
		leaseID,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) RefreshOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) (bool, error) {
	if c == nil || c.rdb == nil || apiKeyID <= 0 || leaseID == "" {
		return false, nil
	}
	result, err := refreshOpenAIWSIngressLeaseScript.Run(
		ctx,
		c.rdb,
		[]string{openAIWSIngressLeaseKey(apiKeyID)},
		openAIWSIngressLeaseTTLSeconds,
		leaseID,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *concurrencyCache) ReleaseOpenAIWSIngressLease(ctx context.Context, apiKeyID int64, leaseID string) error {
	if c == nil || c.rdb == nil || apiKeyID <= 0 || leaseID == "" {
		return nil
	}
	return c.rdb.ZRem(ctx, openAIWSIngressLeaseKey(apiKeyID), leaseID).Err()
}

func (c *concurrencyCache) AcquireLiveLease(
	ctx context.Context,
	accountID int64,
	accountMax int,
	userID int64,
	userMax int,
	apiKeyID int64,
	leaseID string,
	replacingRegularSlots bool,
) (bool, error) {
	if c == nil || c.rdb == nil || accountID <= 0 || userID <= 0 || apiKeyID <= 0 || leaseID == "" {
		return false, nil
	}
	replacing := 0
	if replacingRegularSlots {
		replacing = 1
	}
	result, err := acquireLiveLeaseScript.Run(ctx, c.rdb, []string{
		accountSlotKey(accountID),
		liveAccountSlotKey(accountID),
		userSlotKey(userID),
		liveUserSlotKey(userID),
		liveAPIKeySlotKey(apiKeyID),
	}, accountMax, userMax, liveLeaseTTLSeconds, leaseID, replacing).Int()
	return result == 1, err
}

func (c *concurrencyCache) RefreshLiveLease(ctx context.Context, accountID, userID, apiKeyID int64, leaseID string) (bool, error) {
	if c == nil || c.rdb == nil || leaseID == "" {
		return false, nil
	}
	result, err := refreshLiveLeaseScript.Run(ctx, c.rdb, []string{
		liveAccountSlotKey(accountID),
		liveUserSlotKey(userID),
		liveAPIKeySlotKey(apiKeyID),
	}, liveLeaseTTLSeconds, leaseID).Int()
	return result == 1, err
}

func (c *concurrencyCache) ReleaseLiveLease(ctx context.Context, accountID, userID, apiKeyID int64, leaseID string) error {
	if c == nil || c.rdb == nil || leaseID == "" {
		return nil
	}
	pipe := c.rdb.TxPipeline()
	pipe.ZRem(ctx, liveAccountSlotKey(accountID), leaseID)
	pipe.ZRem(ctx, liveUserSlotKey(userID), leaseID)
	pipe.ZRem(ctx, liveAPIKeySlotKey(apiKeyID), leaseID)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *concurrencyCache) GetAPIKeyConcurrencyBatch(ctx context.Context, apiKeyIDs []int64) (map[int64]int, error) {
	if len(apiKeyIDs) == 0 {
		return map[int64]int{}, nil
	}

	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("redis TIME: %w", err)
	}
	cutoffTime := now.Unix() - int64(c.slotTTLSeconds)

	pipe := c.rdb.Pipeline()
	type apiKeyCmd struct {
		apiKeyID int64
		zcardCmd *redis.IntCmd
		liveCmd  *redis.IntCmd
	}
	cmds := make([]apiKeyCmd, 0, len(apiKeyIDs))
	for _, apiKeyID := range apiKeyIDs {
		slotKey := apiKeySlotKeyPrefix + strconv.FormatInt(apiKeyID, 10)
		liveKey := liveAPIKeySlotKeyPrefix + strconv.FormatInt(apiKeyID, 10)
		pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
		pipe.ZRemRangeByScore(ctx, liveKey, "-inf", strconv.FormatInt(now.Unix()-liveLeaseTTLSeconds, 10))
		cmds = append(cmds, apiKeyCmd{
			apiKeyID: apiKeyID,
			zcardCmd: pipe.ZCard(ctx, slotKey),
			liveCmd:  pipe.ZCard(ctx, liveKey),
		})
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}

	result := make(map[int64]int, len(apiKeyIDs))
	for _, cmd := range cmds {
		result[cmd.apiKeyID] = int(cmd.zcardCmd.Val() + cmd.liveCmd.Val())
	}
	return result, nil
}

// Wait queue operations

func (c *concurrencyCache) IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	incremented, _, _, err := c.IncrementWaitCountWithState(ctx, userID, maxWait)
	return incremented, err
}

func (c *concurrencyCache) IncrementWaitCountWithState(ctx context.Context, userID int64, maxWait int) (bool, int, time.Time, error) {
	key := waitQueueKey(userID)
	result, now, current, err := runScriptInt64Triple(ctx, c.rdb, incrementWaitScript, []string{key}, maxWait, c.waitQueueTTLSeconds)
	if err != nil {
		return false, 0, time.Time{}, err
	}
	if result == 1 {
		// 等待队列也会让用户保持“活跃”，否则槽位为 0 时后台任务可能漏看等待计数。
		c.touchActiveIndexAt(ctx, userActiveIndexKey, userID, now+int64(c.waitQueueTTLSeconds))
	}
	return result == 1, int(current), time.Unix(now, 0).UTC(), nil
}

func (c *concurrencyCache) DecrementWaitCount(ctx context.Context, userID int64) error {
	_, _, err := c.DecrementWaitCountWithState(ctx, userID)
	return err
}

func (c *concurrencyCache) DecrementWaitCountWithState(ctx context.Context, userID int64) (int, time.Time, error) {
	key := waitQueueKey(userID)
	remaining, now, err := runScriptInt64Pair(ctx, c.rdb, decrementWaitScript, []string{key})
	if err == nil {
		// 等待数减少后重新判断是否还需要保留索引。
		c.refreshUserActiveIndex(ctx, userID)
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	return int(remaining), time.Unix(now, 0).UTC(), nil
}

// Account wait queue operations

func (c *concurrencyCache) IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error) {
	key := accountWaitKey(accountID)
	result, now, err := runScriptInt64Pair(ctx, c.rdb, incrementAccountWaitScript, []string{key}, maxWait, c.waitQueueTTLSeconds)
	if err != nil {
		return false, err
	}
	if result == 1 {
		// 账号级等待队列同样写入账号活跃索引，供负载查询和清理任务使用。
		c.touchActiveIndexAt(ctx, accountActiveIndexKey, accountID, now+int64(c.waitQueueTTLSeconds))
	}
	return result == 1, nil
}

func (c *concurrencyCache) DecrementAccountWaitCount(ctx context.Context, accountID int64) error {
	key := accountWaitKey(accountID)
	_, err := decrementWaitScript.Run(ctx, c.rdb, []string{key}).Result()
	if err == nil {
		// 等待计数归零后索引需要同步删除，避免后台任务反复处理空账号。
		c.refreshAccountActiveIndex(ctx, accountID)
	}
	return err
}

func (c *concurrencyCache) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	key := accountWaitKey(accountID)
	val, err := c.rdb.Get(ctx, key).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return val, nil
}

func (c *concurrencyCache) GetAccountsLoadBatch(ctx context.Context, accounts []service.AccountWithConcurrency) (map[int64]*service.AccountLoadInfo, error) {
	if len(accounts) == 0 {
		return map[int64]*service.AccountLoadInfo{}, nil
	}

	// 使用 Pipeline 替代 Lua 脚本，兼容 Redis Cluster（Lua 内动态拼 key 会 CROSSSLOT）。
	// 每个账号执行 3 个命令：ZREMRANGEBYSCORE（清理过期）、ZCARD（并发数）、GET（等待数）。
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("redis TIME: %w", err)
	}
	cutoffTime := now.Unix() - int64(c.slotTTLSeconds)

	pipe := c.rdb.Pipeline()

	type accountCmds struct {
		id             int64
		maxConcurrency int
		zcardCmd       *redis.IntCmd
		liveCmd        *redis.IntCmd
		getCmd         *redis.StringCmd
	}
	cmds := make([]accountCmds, 0, len(accounts))
	for _, acc := range accounts {
		slotKey := accountSlotKeyPrefix + strconv.FormatInt(acc.ID, 10)
		liveKey := liveAccountSlotKeyPrefix + strconv.FormatInt(acc.ID, 10)
		waitKey := accountWaitKeyPrefix + strconv.FormatInt(acc.ID, 10)
		pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
		pipe.ZRemRangeByScore(ctx, liveKey, "-inf", strconv.FormatInt(now.Unix()-liveLeaseTTLSeconds, 10))
		ac := accountCmds{
			id:             acc.ID,
			maxConcurrency: acc.MaxConcurrency,
			zcardCmd:       pipe.ZCard(ctx, slotKey),
			liveCmd:        pipe.ZCard(ctx, liveKey),
			getCmd:         pipe.Get(ctx, waitKey),
		}
		cmds = append(cmds, ac)
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}

	loadMap := make(map[int64]*service.AccountLoadInfo, len(accounts))
	for _, ac := range cmds {
		currentConcurrency := int(ac.zcardCmd.Val() + ac.liveCmd.Val())
		waitingCount := 0
		if v, err := ac.getCmd.Int(); err == nil {
			waitingCount = v
		}
		loadRate := 0
		if ac.maxConcurrency > 0 {
			loadRate = (currentConcurrency + waitingCount) * 100 / ac.maxConcurrency
		}
		loadMap[ac.id] = &service.AccountLoadInfo{
			AccountID:          ac.id,
			CurrentConcurrency: currentConcurrency,
			WaitingCount:       waitingCount,
			LoadRate:           loadRate,
		}
	}

	return loadMap, nil
}

func (c *concurrencyCache) GetUsersLoadBatch(ctx context.Context, users []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	if len(users) == 0 {
		return map[int64]*service.UserLoadInfo{}, nil
	}

	// 使用 Pipeline 替代 Lua 脚本，兼容 Redis Cluster。
	now, err := c.rdb.Time(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("redis TIME: %w", err)
	}
	cutoffTime := now.Unix() - int64(c.slotTTLSeconds)

	pipe := c.rdb.Pipeline()

	type userCmds struct {
		id             int64
		maxConcurrency int
		zcardCmd       *redis.IntCmd
		liveCmd        *redis.IntCmd
		getCmd         *redis.StringCmd
	}
	cmds := make([]userCmds, 0, len(users))
	for _, u := range users {
		slotKey := userSlotKeyPrefix + strconv.FormatInt(u.ID, 10)
		liveKey := liveUserSlotKeyPrefix + strconv.FormatInt(u.ID, 10)
		waitKey := waitQueueKeyPrefix + strconv.FormatInt(u.ID, 10)
		pipe.ZRemRangeByScore(ctx, slotKey, "-inf", strconv.FormatInt(cutoffTime, 10))
		pipe.ZRemRangeByScore(ctx, liveKey, "-inf", strconv.FormatInt(now.Unix()-liveLeaseTTLSeconds, 10))
		uc := userCmds{
			id:             u.ID,
			maxConcurrency: u.MaxConcurrency,
			zcardCmd:       pipe.ZCard(ctx, slotKey),
			liveCmd:        pipe.ZCard(ctx, liveKey),
			getCmd:         pipe.Get(ctx, waitKey),
		}
		cmds = append(cmds, uc)
	}

	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("pipeline exec: %w", err)
	}

	loadMap := make(map[int64]*service.UserLoadInfo, len(users))
	for _, uc := range cmds {
		currentConcurrency := int(uc.zcardCmd.Val() + uc.liveCmd.Val())
		waitingCount := 0
		if v, err := uc.getCmd.Int(); err == nil {
			waitingCount = v
		}
		loadRate := 0
		if uc.maxConcurrency > 0 {
			loadRate = (currentConcurrency + waitingCount) * 100 / uc.maxConcurrency
		}
		loadMap[uc.id] = &service.UserLoadInfo{
			UserID:             uc.id,
			CurrentConcurrency: currentConcurrency,
			WaitingCount:       waitingCount,
			LoadRate:           loadRate,
		}
	}

	return loadMap, nil
}

func (c *concurrencyCache) CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error {
	key := accountSlotKey(accountID)
	_, err := cleanupExpiredSlotsScript.Run(ctx, c.rdb, []string{key}, c.slotTTLSeconds).Result()
	if err == nil {
		// 单账号清理后同步索引，保持后台批量清理的候选集准确。
		c.refreshAccountActiveIndex(ctx, accountID)
	}
	return err
}

// CleanupExpiredAccountSlotKeys 处理账号与用户两个活跃索引中已到期的候选。
// （方法名中的 Account 是历史遗留，保留以避免接口变更；实际同时回收两个索引，
// 否则 user 索引的过期成员没有任何清理路径，会无界累积。）
func (c *concurrencyCache) CleanupExpiredAccountSlotKeys(ctx context.Context) error {
	if err := c.reconcileExpiredIndexCandidates(ctx, accountSlotIndex); err != nil {
		return err
	}
	return c.reconcileExpiredIndexCandidates(ctx, userSlotIndex)
}

// reconcileExpiredIndexCandidates 处理单个活跃索引中 score 已到期的候选：
// 无真实负载则移除 member；仍有负载则按真实负载批量刷新 score。
func (c *concurrencyCache) reconcileExpiredIndexCandidates(ctx context.Context, spec slotIndexSpec) error {
	now, err := c.redisUnixSeconds(ctx)
	if err != nil {
		return err
	}
	members, err := c.rdb.ZRangeByScore(ctx, spec.indexKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now, 10),
		Count: activeIndexCleanupBatchSize,
	}).Result()
	if err != nil {
		return fmt.Errorf("read expired index %s: %w", spec.indexKey, err)
	}

	loads, staleMembers, err := c.readIndexLoads(ctx, spec, members, now)
	if err != nil {
		return err
	}
	refreshed := make([]redis.Z, 0, len(loads))
	for _, load := range loads {
		if load.slotCount == 0 && load.waitCount <= 0 {
			// 真实槽位和等待数都为空，说明这个索引 member 已经完成使命。
			staleMembers = append(staleMembers, load.member)
			continue
		}
		refreshed = append(refreshed, redis.Z{
			Score:  float64(now + int64(c.activeIndexTTL(load.slotCount, load.waitCount))),
			Member: load.member,
		})
	}
	if len(refreshed) > 0 {
		if err := c.rdb.ZAdd(ctx, spec.indexKey, refreshed...).Err(); err != nil {
			logger.LegacyPrintf("repository.concurrency", "Warning: refresh %d active index members in %s failed: %v", len(refreshed), spec.indexKey, err)
		}
	}
	c.removeActiveIndexMembers(ctx, spec.indexKey, staleMembers)
	return nil
}

// CleanupStaleProcessSlots 启动时清理非当前进程前缀的槽位。
// 清理范围来自活跃索引（含 score 已过期的成员——它们往往正是崩溃进程留下的残留），
// 避免在 Redis 上 SCAN 全部 concurrency:* 键；另有一次性迁移清扫兜底索引机制上线前的遗留等待计数。
// API Key 槽位（concurrency:api_key:*）是 stats-only 数据：每次 Track/读取都会按分数
// 裁剪过期成员，key 自带 TTL，可在一个 slot TTL 内自愈，因此不参与启动清理。
func (c *concurrencyCache) CleanupStaleProcessSlots(ctx context.Context, activeRequestPrefix string) error {
	if activeRequestPrefix == "" {
		return nil
	}
	if err := c.sweepLegacyWaitKeysOnce(ctx); err != nil {
		return err
	}
	if err := c.cleanupStaleRequestBodyAdmissionSlots(ctx, activeRequestPrefix); err != nil {
		return err
	}
	now, err := c.redisUnixSeconds(ctx)
	if err != nil {
		return err
	}

	accountMembers, err := c.allIndexMembers(ctx, accountActiveIndexKey)
	if err != nil {
		return err
	}
	if err := c.cleanupStaleProcessSlotsForIndex(ctx, accountSlotIndex, accountMembers, activeRequestPrefix, now); err != nil {
		return err
	}

	userMembers, err := c.allIndexMembers(ctx, userActiveIndexKey)
	if err != nil {
		return err
	}
	return c.cleanupStaleProcessSlotsForIndex(ctx, userSlotIndex, userMembers, activeRequestPrefix, now)
}

func requestBodyAdmissionMemberHasProcessPrefix(member, activeRequestPrefix string) bool {
	for _, part := range strings.Split(member, ":") {
		if strings.HasPrefix(part, activeRequestPrefix) {
			return true
		}
	}
	return false
}

func requestBodyAdmissionUserIDFromKey(key, keyPrefix string) (int64, bool) {
	rawID := strings.TrimPrefix(key, keyPrefix)
	if rawID == key || rawID == "" || strings.Contains(rawID, ":") {
		return 0, false
	}
	userID, err := strconv.ParseInt(rawID, 10, 64)
	return userID, err == nil && userID > 0
}

// cleanupStaleRequestBodyAdmissionSlots removes leases left by a terminated
// process. These keys are not covered by the account/user active indexes and a
// stale global recovery member would otherwise block compaction for one full
// slot TTL after an OOM restart.
func (c *concurrencyCache) cleanupStaleRequestBodyAdmissionSlots(ctx context.Context, activeRequestPrefix string) error {
	activeUsers := make(map[int64]struct{})
	cleanupZSet := func(key string) (bool, error) {
		members, err := c.rdb.ZRange(ctx, key, 0, -1).Result()
		if err != nil {
			return false, err
		}
		stale := make([]any, 0, len(members))
		hasActive := false
		for _, member := range members {
			if requestBodyAdmissionMemberHasProcessPrefix(member, activeRequestPrefix) {
				hasActive = true
				continue
			}
			stale = append(stale, member)
		}
		if len(stale) > 0 {
			if err := c.rdb.ZRem(ctx, key, stale...).Err(); err != nil {
				return false, err
			}
		}
		return hasActive, nil
	}

	patterns := []struct {
		pattern       string
		userKeyPrefix string
		waitKeys      bool
	}{
		{pattern: requestBodyLaneKeyPrefix + "{admission}:*", userKeyPrefix: requestBodyLaneKeyPrefix + "{admission}:user:"},
		{pattern: requestBodyLaneWaitKeyPrefix + "{admission}:*", userKeyPrefix: requestBodyLaneWaitKeyPrefix + "{admission}:user:", waitKeys: true},
	}
	for _, item := range patterns {
		var cursor uint64
		for {
			keys, next, err := c.rdb.Scan(ctx, cursor, item.pattern, 200).Result()
			if err != nil {
				return fmt.Errorf("scan request body admission keys %s: %w", item.pattern, err)
			}
			for _, key := range keys {
				hasActive := false
				if item.waitKeys && strings.HasPrefix(key, item.userKeyPrefix) {
					value, getErr := c.rdb.Get(ctx, key).Result()
					if getErr != nil && !errors.Is(getErr, redis.Nil) {
						return fmt.Errorf("read request body wait key %s: %w", key, getErr)
					}
					hasActive = getErr == nil && requestBodyAdmissionMemberHasProcessPrefix(value, activeRequestPrefix)
					if getErr == nil && !hasActive {
						if err := c.rdb.Del(ctx, key).Err(); err != nil {
							return fmt.Errorf("delete stale request body wait key %s: %w", key, err)
						}
					}
				} else {
					hasActive, err = cleanupZSet(key)
					if err != nil {
						return fmt.Errorf("clean stale request body admission key %s: %w", key, err)
					}
				}
				if hasActive {
					if userID, ok := requestBodyAdmissionUserIDFromKey(key, item.userKeyPrefix); ok {
						activeUsers[userID] = struct{}{}
					}
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}

	if err := c.rdb.Del(ctx, requestBodyActiveIndexKey).Err(); err != nil {
		return fmt.Errorf("reset request body active index: %w", err)
	}
	if len(activeUsers) == 0 {
		return nil
	}
	now, err := c.redisUnixSeconds(ctx)
	if err != nil {
		return err
	}
	entries := make([]redis.Z, 0, len(activeUsers))
	for userID := range activeUsers {
		entries = append(entries, redis.Z{
			Score:  float64(now + int64(c.slotTTLSeconds)),
			Member: strconv.FormatInt(userID, 10),
		})
	}
	if err := c.rdb.ZAdd(ctx, requestBodyActiveIndexKey, entries...).Err(); err != nil {
		return fmt.Errorf("rebuild request body active index: %w", err)
	}
	return nil
}

// sweepLegacyWaitKeysOnce 一次性清扫活跃索引机制上线前遗留的等待计数键。
// 等待计数在有流量时会不断刷新 TTL、无法自然过期，而索引不认识旧键，
// 因此这里例外地做一次 SCAN，用 marker 键保证整个 Redis 数据生命周期内只执行一次。
// 先清扫后写 marker：清扫失败时下次启动会重试；并发实例重复清扫是幂等的。
func (c *concurrencyCache) sweepLegacyWaitKeysOnce(ctx context.Context) error {
	exists, err := c.rdb.Exists(ctx, legacyWaitSweepMarkerKey).Result()
	if err != nil {
		return fmt.Errorf("check legacy wait sweep marker: %w", err)
	}
	if exists > 0 {
		return nil
	}
	for _, pattern := range []string{accountWaitKeyPrefix + "*", waitQueueKeyPrefix + "*"} {
		var cursor uint64
		for {
			keys, next, err := c.rdb.Scan(ctx, cursor, pattern, 200).Result()
			if err != nil {
				return fmt.Errorf("scan legacy wait keys %s: %w", pattern, err)
			}
			if len(keys) > 0 {
				if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
					return fmt.Errorf("delete legacy wait keys: %w", err)
				}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	if err := c.rdb.Set(ctx, legacyWaitSweepMarkerKey, "1", 0).Err(); err != nil {
		return fmt.Errorf("set legacy wait sweep marker: %w", err)
	}
	return nil
}

// allIndexMembers 返回索引中全部 member（含 score 已过期的）。
// 启动清理必须覆盖过期成员：长时间停机后 score 过期的候选恰恰最可能持有死进程残留。
func (c *concurrencyCache) allIndexMembers(ctx context.Context, indexKey string) ([]string, error) {
	members, err := c.rdb.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read active index %s: %w", indexKey, err)
	}
	return members, nil
}

// cleanupStaleProcessSlotsForIndex 逐个处理索引中的账号/用户。
// Lua 脚本一次只碰一个槽位 key，兼容 Redis Cluster，随后删除重启后已失效的等待计数；
// 索引 member 的去留由脚本返回的剩余槽位数决定，最后批量写回。
func (c *concurrencyCache) cleanupStaleProcessSlotsForIndex(
	ctx context.Context,
	spec slotIndexSpec,
	members []string,
	activeRequestPrefix string,
	now int64,
) error {
	staleMembers := make([]string, 0)
	refreshed := make([]redis.Z, 0)
	for _, member := range members {
		id, err := strconv.ParseInt(member, 10, 64)
		if err != nil || id <= 0 {
			staleMembers = append(staleMembers, member)
			continue
		}

		_, remaining, err := runScriptInt64Pair(ctx, c.rdb, startupCleanupSlotScript, []string{spec.slotKey(id)}, activeRequestPrefix, c.slotTTLSeconds)
		if err != nil {
			return fmt.Errorf("cleanup stale process slots %s: %w", spec.slotKey(id), err)
		}
		// 等待计数属于已死进程，直接删除；剩余槽位（当前进程前缀）决定索引 member 去留。
		if err := c.rdb.Del(ctx, spec.waitKey(id)).Err(); err != nil {
			return fmt.Errorf("delete stale wait key %s: %w", spec.waitKey(id), err)
		}
		if remaining > 0 {
			refreshed = append(refreshed, redis.Z{
				Score:  float64(now + int64(c.slotTTLSeconds)),
				Member: member,
			})
		} else {
			staleMembers = append(staleMembers, member)
		}
	}
	if len(refreshed) > 0 {
		if err := c.rdb.ZAdd(ctx, spec.indexKey, refreshed...).Err(); err != nil {
			logger.LegacyPrintf("repository.concurrency", "Warning: refresh %d active index members in %s failed: %v", len(refreshed), spec.indexKey, err)
		}
	}
	c.removeActiveIndexMembers(ctx, spec.indexKey, staleMembers)
	return nil
}
