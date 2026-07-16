package service

import (
	"context"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// RequestArchiveEntry 是一条待存档的请求记录。
type RequestArchiveEntry struct {
	ID        int64
	CreatedAt time.Time
	RequestID string
	UserID    int64
	UserEmail string
	APIKeyID  int64
	APIKeyName string
	GroupID   *int64
	Endpoint  string
	Protocol  string
	Model     string
	IPAddress string
	PromptText string
	Truncated bool
}

// RequestArchiveConfig 控制请求存档行为。
type RequestArchiveConfig struct {
	Enabled       bool // 总开关,默认关闭
	RetentionDays int  // 保留天数,默认 30
}

// RequestArchiveRepository 是存档表的数据访问接口。
type RequestArchiveRepository interface {
	BatchInsert(ctx context.Context, entries []RequestArchiveEntry) error
	List(ctx context.Context, page, pageSize int, search string, userID *int64, apiKeyID *int64, startDate, endDate *time.Time) ([]RequestArchiveEntry, int64, error)
	GetByID(ctx context.Context, id int64) (*RequestArchiveEntry, error)
	CleanupOlderThan(ctx context.Context, before time.Time) (int64, error)
}

const (
	requestArchiveQueueCap      = 4096          // 内存队列硬上限,防止爆内存(约 8MB)
	requestArchiveBatchSize     = 100           // 批量写入条数
	requestArchiveFlushInterval = 5 * time.Second // 最大落库间隔
	requestArchiveTextLimit     = 8 * 1024      // 单条 prompt 截断上限 8KB
	requestArchiveDefaultRetain = 30            // 默认保留 30 天
)

// RequestArchiveService 异步接收请求文本并存档,不阻塞网关请求。
type RequestArchiveService struct {
	repo     RequestArchiveRepository
	queue    chan RequestArchiveEntry
	stopCh   chan struct{}
	wg       sync.WaitGroup
	cfgStore func() RequestArchiveConfig
}

// NewRequestArchiveService 创建存档服务。cfgStore 返回当前配置(允许动态开关)。
func NewRequestArchiveService(repo RequestArchiveRepository, cfgStore func() RequestArchiveConfig) *RequestArchiveService {
	svc := &RequestArchiveService{
		repo:     repo,
		queue:    make(chan RequestArchiveEntry, requestArchiveQueueCap),
		stopCh:   make(chan struct{}),
		cfgStore: cfgStore,
	}
	if repo != nil {
		svc.wg.Add(2)
		go svc.flushWorker()
		go svc.cleanupWorker()
	}
	return svc
}

// Archive 异步提交一条请求记录。非阻塞:队列满时丢弃(不影响请求)。
func (s *RequestArchiveService) Archive(entry RequestArchiveEntry) {
	if s == nil || s.repo == nil {
		return
	}
	// 截断超长文本
	if len(entry.PromptText) > requestArchiveTextLimit {
		entry.PromptText = entry.PromptText[:requestArchiveTextLimit]
		entry.Truncated = true
	}
	select {
	case s.queue <- entry:
	default:
		// 队列满,丢弃旧数据不入队,绝不阻塞网关
	}
}

// Stop 优雅关闭:等待队列排空。
func (s *RequestArchiveService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *RequestArchiveService) flushWorker() {
	defer s.wg.Done()
	batch := make([]RequestArchiveEntry, 0, requestArchiveBatchSize)
	ticker := time.NewTicker(requestArchiveFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case entry := <-s.queue:
			batch = append(batch, entry)
			if len(batch) >= requestArchiveBatchSize {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
		case <-s.stopCh:
			// 关闭前排空队列
			for len(batch) > 0 {
				s.flushBatch(batch)
				batch = batch[:0]
			}
			for {
				select {
				case entry := <-s.queue:
					batch = append(batch, entry)
					if len(batch) >= requestArchiveBatchSize {
						s.flushBatch(batch)
						batch = batch[:0]
					}
				default:
					if len(batch) > 0 {
						s.flushBatch(batch)
					}
					return
				}
			}
		}
	}
}

func (s *RequestArchiveService) flushBatch(batch []RequestArchiveEntry) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.repo.BatchInsert(ctx, batch); err != nil {
		logger.LegacyPrintf("service.request_archive", "batch insert %d entries failed: %v", len(batch), err)
	}
}

func (s *RequestArchiveService) cleanupWorker() {
	defer s.wg.Done()
	// 启动后延迟 5 分钟再首次清理
	timer := time.NewTimer(5 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			s.runCleanupOnce()
			timer.Reset(24 * time.Hour) // 每天清理一次
		case <-s.stopCh:
			return
		}
	}
}

func (s *RequestArchiveService) runCleanupOnce() {
	if s.repo == nil {
		return
	}
	cfg := requestArchiveCurrentConfig(s.cfgStore)
	days := cfg.RetentionDays
	if days <= 0 {
		days = requestArchiveDefaultRetain
	}
	before := time.Now().AddDate(0, 0, -days)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	deleted, err := s.repo.CleanupOlderThan(ctx, before)
	if err != nil {
		logger.LegacyPrintf("service.request_archive", "cleanup older than %v failed: %v", before, err)
		return
	}
	if deleted > 0 {
		logger.LegacyPrintf("service.request_archive", "cleanup deleted %d entries older than %d days", deleted, days)
	}
}

// requestArchiveCurrentConfig 安全读取配置。
func requestArchiveCurrentConfig(cfgStore func() RequestArchiveConfig) RequestArchiveConfig {
	if cfgStore == nil {
		return RequestArchiveConfig{Enabled: false, RetentionDays: requestArchiveDefaultRetain}
	}
	cfg := cfgStore()
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = requestArchiveDefaultRetain
	}
	return cfg
}

// IsEnabled 报告存档是否启用。
func (s *RequestArchiveService) IsEnabled() bool {
	if s == nil || s.cfgStore == nil {
		return false
	}
	return s.cfgStore().Enabled
}

// Repository 返回底层 repo(供 handler 查询)。
func (s *RequestArchiveService) Repository() RequestArchiveRepository {
	if s == nil {
		return nil
	}
	return s.repo
}
