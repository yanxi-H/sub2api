//go:build unit

// API Key 服务创建/归属分配的单元测试
// 测试 APIKeyService.Create 方法：管理员代建时通过 CreateAPIKeyRequest.UserID
// 指定归属用户；未指定时归属调用者本人。归属修改（Update 改 owner）已下线，
// UpdateAPIKeyRequest 不再承载 UserID。

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// multiUserRepoStub 是按 id 映射返回用户的 UserRepository 桩。
// 与 admin_service_delete_test.go 里的单用户 userRepoStub 不同，它支持
// 预置多个用户，便于测试「目标用户 ≠ 调用者」的代建场景。
type multiUserRepoStub struct {
	UserRepository
	users      map[int64]*User
	getByIDErr error
}

func (s *multiUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if s.getByIDErr != nil {
		return nil, s.getByIDErr
	}
	if u, ok := s.users[id]; ok {
		clone := *u
		return &clone, nil
	}
	return nil, errors.New("user not found")
}

// createApiKeyRepoStub 是专用于 Create 测试的 APIKeyRepository 桩。
// 它记录被 Create 的 Key（用于断言归属 UserID），并支持 ExistsByKey 返回 false。
type createApiKeyRepoStub struct {
	listAllApiKeyRepoStub
	createdKeys []APIKey // 记录被 Create 写入的 Key
	createErr   error
}

func (s *createApiKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.createErr != nil {
		return s.createErr
	}
	if key != nil {
		s.createdKeys = append(s.createdKeys, *key)
	}
	return nil
}

func (s *createApiKeyRepoStub) ExistsByKey(_ context.Context, _ string) (bool, error) {
	return false, nil // 自定义 key 总是「不存在」，允许创建
}

// createGroupRepoStub 是 Create 测试用的 GroupRepository 最小桩。
// GetByID 返回一个有效 group；exclusive 字段控制 canUserBindGroup 的走向。
type createGroupRepoStub struct {
	GroupRepository
	exclusive bool
}

func (s *createGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id, IsExclusive: s.exclusive}, nil
}

func newCreateService(repo APIKeyRepository, userRepo UserRepository) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   userRepo,
		groupRepo:  &createGroupRepoStub{},
		cache:      &apiKeyCacheStub{},
		cfg:        &config.Config{}, // GenerateKey 用 s.cfg.Default.APIKeyPrefix（空 → fallback "sk-"）
	}
}

// TestApiKeyService_Create_AssignsToRequestedUser 验证：请求里指定 UserID 时
// （管理员代建，端点在 handler 层已做 admin 门卫），Key 归属目标用户，
// 而非调用者本人。
func TestApiKeyService_Create_AssignsToRequestedUser(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			42: {ID: 42, Username: "alice"},
		},
	}
	svc := newCreateService(repo, userRepo)

	// 调用者 999 请求把 Key 建给目标用户 42
	target := int64(42)
	key, err := svc.Create(context.Background(), 999, CreateAPIKeyRequest{Name: "for-alice", UserID: &target})
	require.NoError(t, err)
	require.Len(t, repo.createdKeys, 1)
	// 核心断言：归属是目标用户 42，不是调用者
	require.Equal(t, int64(42), key.UserID)
	require.Equal(t, int64(42), repo.createdKeys[0].UserID)
	require.Equal(t, "for-alice", key.Name)
}

// TestApiKeyService_Create_NormalUser_AssignsToSelf 验证：未指定 UserID 时，
// 归属就是传入的 userID（调用者本人）。
func TestApiKeyService_Create_NormalUser_AssignsToSelf(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			7: {ID: 7, Username: "bob"},
		},
	}
	svc := newCreateService(repo, userRepo)

	key, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{Name: "self-key"})
	require.NoError(t, err)
	require.Equal(t, int64(7), key.UserID) // 归属调用者自己
}

// TestApiKeyService_Create_TargetUserNotFound 验证：指定的目标用户不存在时，
// 返回错误（userRepo.GetByID 失败），且不创建 Key。
func TestApiKeyService_Create_TargetUserNotFound(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{}, // 空：目标用户 999 不存在
	}
	svc := newCreateService(repo, userRepo)

	target := int64(999)
	_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{Name: "ghost", UserID: &target})
	require.Error(t, err)
	require.Empty(t, repo.createdKeys) // 用户不存在，不应创建 Key
}

// TestApiKeyService_Create_TargetUserGroupCheckRestricted 验证：为目标用户
// 指定其无权绑定的专属分组时返回 GROUP_NOT_ALLOWED。归属分配功能重构后
// 不再对「管理员代建」跳过 canUserBindGroup，校验作用于目标用户。
func TestApiKeyService_Create_TargetUserGroupCheckRestricted(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			5: {ID: 5, Username: "carol"}, // 无 AllowedGroups，无法绑定专属分组
		},
	}
	svc := newCreateService(repo, userRepo)
	svc.groupRepo = &createGroupRepoStub{exclusive: true}
	gid := int64(100)
	target := int64(5)
	_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{
		Name:    "restricted-group",
		UserID:  &target,
		GroupID: &gid,
	})
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	require.Empty(t, repo.createdKeys)
}
