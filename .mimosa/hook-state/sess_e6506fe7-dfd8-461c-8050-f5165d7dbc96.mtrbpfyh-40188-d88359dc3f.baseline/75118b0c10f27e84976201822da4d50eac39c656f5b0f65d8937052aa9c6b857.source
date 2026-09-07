//go:build unit

// API Key 服务创建/分配方法的单元测试
// 测试 APIKeyService.Create / CreateAsAdmin 方法，重点验证管理员代建时
// 能把 Key 分配给指定目标用户，以及 UpdateAsAdmin 能修改归属用户。

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
// 预置多个用户，便于测试「目标用户 ≠ 调用者」的代建/改归属场景。
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

// noopGroupRepo 是 GroupRepository 的最小桩。管理员模式跳过分组校验，
// 因此即使不预置分组也不会被调用；这里仅满足接口与 service 字段赋值。
type noopGroupRepo struct {
	GroupRepository
}

func newCreateService(repo APIKeyRepository, userRepo UserRepository) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: repo,
		userRepo:   userRepo,
		groupRepo:  &noopGroupRepo{},
		cache:      &apiKeyCacheStub{},
		cfg:        &config.Config{}, // GenerateKey 用 s.cfg.Default.APIKeyPrefix（空 → fallback "sk-"）
	}
}

// TestApiKeyService_CreateAsAdmin_AssignsToTargetUser 验证：管理员代建时，
// 传入的目标 userID 决定 Key 归属，而非调用者本人。
func TestApiKeyService_CreateAsAdmin_AssignsToTargetUser(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			42: {ID: 42, Username: "alice"},
		},
	}
	svc := newCreateService(repo, userRepo)

	// 管理员调用，目标用户 42（非调用者 999）
	key, err := svc.CreateAsAdmin(context.Background(), 42, CreateAPIKeyRequest{Name: "for-alice"})
	require.NoError(t, err)
	require.Len(t, repo.createdKeys, 1)
	// 核心断言：归属是目标用户 42，不是调用者
	require.Equal(t, int64(42), key.UserID)
	require.Equal(t, int64(42), repo.createdKeys[0].UserID)
	require.Equal(t, "for-alice", key.Name)
}

// TestApiKeyService_Create_NormalUser_AssignsToSelf 验证：普通 Create 路径，
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

// TestApiKeyService_CreateAsAdmin_TargetUserNotFound 验证：管理员指定的目标用户
// 不存在时，返回错误（userRepo.GetByID 失败），且不创建 Key。
func TestApiKeyService_CreateAsAdmin_TargetUserNotFound(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{}, // 空：目标用户 999 不存在
	}
	svc := newCreateService(repo, userRepo)

	_, err := svc.CreateAsAdmin(context.Background(), 999, CreateAPIKeyRequest{Name: "ghost"})
	require.Error(t, err)
	require.Empty(t, repo.createdKeys) // 用户不存在，不应创建 Key
}

// TestApiKeyService_CreateAsAdmin_BypassesGroupCheck 验证：管理员代建时
// 跳过 canUserBindGroup 校验。groupRepo.GetByID 仍会被调用（取 group 对象），
// 但 canUserBindGroup 这道权限校验对管理员跳过，所以即便用户本无权绑定该分组也能成功。
// 这里让 groupRepo.GetByID 返回一个「专属分组」，普通路径会因 canUserBindGroup
// 失败，管理员路径应成功。
func TestApiKeyService_CreateAsAdmin_BypassesGroupCheck(t *testing.T) {
	repo := &createApiKeyRepoStub{}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			5: {ID: 5, Username: "carol"},
		},
	}
	svc := newCreateService(repo, userRepo)
	gid := int64(100)
	key, err := svc.CreateAsAdmin(context.Background(), 5, CreateAPIKeyRequest{
		Name:    "no-group-check",
		GroupID: &gid,
	})
	require.NoError(t, err)
	require.Equal(t, int64(5), key.UserID)
	require.Equal(t, &gid, key.GroupID)
}

// noopGroupRepo.GetByID 返回一个有效 group（管理员路径取 group 对象用）。
func (s *noopGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id}, nil
}

// TestApiKeyService_UpdateAsAdmin_ChangesOwner 验证：管理员能通过
// UpdateAPIKeyRequest.UserID 修改 Key 的归属用户。
func TestApiKeyService_UpdateAsAdmin_ChangesOwner(t *testing.T) {
	newOwner := int64(88)
	repo := &createApiKeyRepoStub{
		listAllApiKeyRepoStub: listAllApiKeyRepoStub{
			apiKeyRepoStub: apiKeyRepoStub{
				apiKey: &APIKey{ID: 70, UserID: 10, Key: "k70", Status: "active"},
			},
		},
	}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{
			88: {ID: 88, Username: "new-owner"},
		},
	}
	svc := newCreateService(repo, userRepo)

	key, err := svc.UpdateAsAdmin(context.Background(), 70, 999, UpdateAPIKeyRequest{UserID: &newOwner})
	require.NoError(t, err)
	// 核心断言：归属从 10 改为 88
	require.Equal(t, int64(88), key.UserID)
	require.Equal(t, []int64{70}, repo.updateCalls)
}

// TestApiKeyService_UpdateAsAdmin_TargetUserNotFound 验证：管理员改归属时，
// 目标用户不存在则返回错误，不修改 Key。
func TestApiKeyService_UpdateAsAdmin_TargetUserNotFound(t *testing.T) {
	newOwner := int64(404)
	repo := &createApiKeyRepoStub{
		listAllApiKeyRepoStub: listAllApiKeyRepoStub{
			apiKeyRepoStub: apiKeyRepoStub{
				apiKey: &APIKey{ID: 70, UserID: 10, Key: "k70", Status: "active"},
			},
		},
	}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{}, // 目标用户 404 不存在
	}
	svc := newCreateService(repo, userRepo)

	_, err := svc.UpdateAsAdmin(context.Background(), 70, 999, UpdateAPIKeyRequest{UserID: &newOwner})
	require.Error(t, err)
	require.Empty(t, repo.updateCalls) // 用户不存在，不应 Update
}

// TestApiKeyService_Update_NormalUser_IgnoresUserID 验证：普通 Update 路径
// 忽略 UserID 字段（普通用户无权改归属）。
func TestApiKeyService_Update_NormalUser_IgnoresUserID(t *testing.T) {
	newOwner := int64(88)
	repo := &createApiKeyRepoStub{
		listAllApiKeyRepoStub: listAllApiKeyRepoStub{
			apiKeyRepoStub: apiKeyRepoStub{
				apiKey: &APIKey{ID: 70, UserID: 10, Key: "k70", Status: "active"},
			},
		},
	}
	userRepo := &multiUserRepoStub{
		users: map[int64]*User{10: {ID: 10}},
	}
	svc := newCreateService(repo, userRepo)

	// 普通用户 10 改自己的 Key，传了 UserID=88 也应被忽略
	key, err := svc.Update(context.Background(), 70, 10, UpdateAPIKeyRequest{UserID: &newOwner})
	require.NoError(t, err)
	require.Equal(t, int64(10), key.UserID) // 归属不变
}
