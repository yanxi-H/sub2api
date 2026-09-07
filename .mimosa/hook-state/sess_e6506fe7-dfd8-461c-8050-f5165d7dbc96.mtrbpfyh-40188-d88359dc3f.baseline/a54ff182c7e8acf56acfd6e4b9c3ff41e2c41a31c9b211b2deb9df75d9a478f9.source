# Sub2API 项目协作规则

## 项目定位

Sub2API 是一个可自托管的全栈 AI API 网关平台。用户通过平台签发的 API Key 访问上游 AI 服务；系统负责鉴权、账号调度、并发和速率控制、用量与计费、支付、协议转发、监控及安全审计。

这是在上游 `Wei-Shaw/sub2api` 基础上维护定制能力的 fork：

- `origin`：`git@github.com:wonderyuan/sub2api.git`
- `upstream`：`https://github.com/Wei-Shaw/sub2api.git`
- Go module 路径仍为 `github.com/Wei-Shaw/sub2api`，不得仅因 fork 仓库地址不同而擅自重命名 module。

处理任务时必须同时考虑用户端、管理端、网关协议、数据库、Redis、部署和现有定制能力的兼容边界，但只读取和修改与当前任务有关的部分。

## 事实来源与文档优先级

事实优先级如下：

1. 用户本次明确说明；
2. 当前代码、配置、测试和实际运行结果；
3. `backend/go.mod`、`frontend/package.json`、锁文件和当前 CI workflow；
4. 与任务相关的 README、`docs/`、`deploy/` 和 OpenSpec 文档；
5. `DEV_GUIDE.md` 等历史开发说明；
6. 中央知识库中的历史内容。

当前已确认的工具链基线：

- 后端：Go 1.26.5、Gin、Ent、Wire；版本以 `backend/go.mod` 和 CI 为准。
- 前端：Node.js 20、pnpm 9、Vue 3、TypeScript、Vite、Pinia、Vue Router、Tailwind CSS、Vitest。
- 数据：PostgreSQL 15+、Redis 7+。
- lint：golangci-lint v2.9；前端使用 ESLint 和 `vue-tsc`。

`README*` 和 `DEV_GUIDE.md` 当前仍存在 Go 1.25.7、旧 fork 地址、旧 lint 版本及特定 Windows 环境等过时信息。遇到冲突时以当前配置和 CI 为准，并明确指出文档漂移；不得把历史机器路径、示例账号或示例密码当作当前环境事实。

## 目录与职责

- `backend/cmd/server/`：后端入口、Wire 装配和版本信息。
- `backend/internal/handler/`：HTTP、SSE、WebSocket 和各上游兼容协议入口；Handler 负责协议适配与协调，不应堆积可复用领域逻辑。
- `backend/internal/service/`：业务服务与跨仓储流程。
- `backend/internal/repository/`：PostgreSQL/Redis 数据访问和事务边界。
- `backend/internal/domain/`、`model/`：领域规则、稳定类型与值对象。
- `backend/internal/platform/`：上游平台实现；新增平台行为应保持既有接口和错误语义。
- `backend/internal/securityaudit/`：Content Moderation 协调、Prompt Audit 和 Prompt Guard；必须遵守下述安全不变量。
- `backend/ent/schema/` 与 `backend/ent/`：Ent schema 与生成代码。
- `backend/migrations/`：按序、不可变的 SQL migration。
- `frontend/src/api/`：后端 API 客户端与 DTO。
- `frontend/src/stores/`：Pinia 全局状态；页面局部状态不要无理由提升到 store。
- `frontend/src/views/`、`components/`、`features/`：页面、共享组件和相对独立的垂直功能。
- `frontend/src/router/`：路由、鉴权和角色 guard。
- `frontend/src/i18n/`：用户可见文案的国际化资源。
- `deploy/`：Docker Compose、安装脚本和运行配置示例。
- `openspec/changes/`：跨模块重要变更的提案、设计、任务和验证证据。

新增代码优先放入现有职责目录并复用既有接口、错误 helper、分页、日志、审计和 UI 基础组件。只有在能消除真实复杂度或已有相同模式时才新增抽象。

## 开发与变更规则

- 先检查 `git status --short`，识别用户已有修改；不得覆盖、清理或回滚来源不明的改动。
- 修改前读取目标模块的实现、相邻测试和相关文档，不默认遍历整个仓库。
- 保持改动聚焦于用户请求；不要顺带格式化无关文件、升级依赖或重构无关模块。
- 修复缺陷时优先补充能稳定复现根因的回归测试，再做最小实现修复。
- 改动公共 interface 时搜索并同步生产实现、Wire provider、测试 stub/mock 和调用方。
- 修改配置时同步检查默认值、校验、环境变量映射、部署示例和前端管理入口，避免只改一层。
- 新增用户可见文案时维护现有语言资源，不在 Vue 模板中散落仅单语言硬编码。
- 新增路由时沿用懒加载和既有 `requiresAuth`、`requiresAdmin` 等 meta/guard 约定，并补充相应路由测试。
- `standard` 与 `simple` 模式存在明确业务差异；涉及计费、余额、SaaS 页面或鉴权时必须覆盖相关模式，不能假定行为相同。
- 当前 Sora 能力在 README 中标为暂不可用；不得仅凭残留代码或配置宣称其已恢复。

## 后端规则

- 使用标准 Go 格式和当前 `.golangci.yml`；不要以降低 lint、安全或架构检查强度来让 CI 通过。
- 错误应沿用现有 domain/handler helper 和协议 envelope；不要把内部错误、上游凭据、节点详情或敏感请求内容直接返回给客户端。
- 数据写入、计费预扣、并发 slot、账号选择和上游调用的顺序具有业务语义。修改网关热路径时必须证明失败、取消、重试和 failover 不会重复计费或泄漏资源。
- Redis 既承担缓存/队列也可能影响跨实例一致性。新增本地缓存或锁时必须说明失效、TTL、多实例和 Redis 不可用时的行为。
- Ent schema 修改后运行生成命令并提交生成结果；Wire 依赖变化后也必须重新生成：

```bash
make -C backend generate
```

- 不手工编辑 `backend/ent/` 或 `backend/cmd/server/wire_gen.go` 中由生成器负责的内容，除非文件明确不是生成产物。

## 数据库迁移规则

- 已在任何环境应用的 migration 永久不可修改、删除、重命名或重排；runner 会校验 SHA256。
- 新变更使用 `backend/migrations/` 当前最大序号之后的新文件，命名为 `NNN_description.sql`。
- migration 应小而聚焦、forward-only，并尽量使用幂等条件；需要回退时新增反向 migration，不在同一文件放可执行的 Down SQL。
- 普通 `*.sql` 在事务中执行。只有 `CREATE/DROP INDEX CONCURRENTLY` 使用 `*_notx.sql`；该文件不得混入其他 DDL/DML 或事务控制语句，并必须使用 `IF EXISTS`/`IF NOT EXISTS`。
- 不通过手工插入 `schema_migrations`、跳过 checksum 或修改历史文件来掩盖 migration 问题。
- 涉及数据回填、约束收紧或大表索引时，必须评估锁表、批量规模、重复执行和旧版本兼容性。

## 前端规则

- 只使用 pnpm 管理前端依赖，不使用 npm/yarn 改写依赖树；修改 `package.json` 时同步更新并提交 `frontend/pnpm-lock.yaml`。
- 默认使用 `pnpm --dir frontend ...` 从仓库根执行命令，或进入 `frontend/` 后运行等价命令。
- API 类型和转换集中在 `src/api/` 或功能模块现有边界，组件不要自行拼接重复请求协议。
- 沿用现有 Pinia、Vue Router、Tailwind 和共享组件模式；不要引入第二套状态管理、路由或样式系统。
- 修改鉴权、支付、设置、管理页或响应式布局时，至少覆盖加载、空数据、失败、权限不足和窄屏状态。
- `pnpm run lint` 会自动修复文件；验证阶段优先使用不会改文件的 `pnpm run lint:check`。

## 协议、安全与隐私不变量

- 保持 OpenAI、Claude、Gemini、媒体接口和 WebSocket 的既有请求/响应 envelope；内部实现变化不得无意制造 breaking change。
- Content Moderation 与 Prompt Audit 是独立能力，不得合并配置、事件表、风险分类或副作用，也不得静默改变既有内容审核行为。
- Prompt Audit 默认关闭。生产 Prompt Guard blocking 未完成真实 async 观测、健康、误报、告警、值班和回滚证据前不得开启。
- Prompt Guard 的 Block/Unavailable/Invalid 必须发生在账号选择、计费和上游副作用之前，并维持零账号、零计费、零上游调用不变量。
- 完整 Prompt 只允许短暂存在于请求内存和 Redis 短 TTL 载荷；不得进入 PostgreSQL、日志、管理 API、前端状态或错误响应。
- 不得提交或记录 API Key、Token、Cookie、OAuth/支付凭据、真实 `.env`、完整生产日志或未脱敏用户数据。示例配置中的密码和 secret 仅是占位值，绝不能用于生产。
- 出站 URL 和审计节点必须保留 SSRF、DNS rebinding、重定向、响应体上限和超时保护；不得为了兼容任意地址而默认放宽。

## OpenSpec 与文档

- 对跨模块、协议、安全、计费、数据模型或重要管理工作流的变更，优先沿用 `openspec/changes/<change-id>/` 的 spec-driven 结构，明确目标、非目标、兼容边界、任务和验证证据。
- 若任务已有 OpenSpec change，实施时同步更新其 tasks、verification 和 implementation evidence，不能只改代码后留下失真的完成状态。
- OpenSpec 中写“通过”必须对应可重复执行的测试、命令、SQL、日志查询或脱敏截图；人工口头确认不能代替证据。
- 文档只记录当前可验证行为。README、部署示例、API 文档和实现发生冲突时，要么在本任务范围内同步修正，要么明确列为待办。

## 构建与验证

验证范围应与改动风险匹配。优先运行目标包/目标测试，再运行相关门禁；不要用一次全仓测试代替对失败路径的针对性验证。

常用命令：

```bash
# 全量构建
make build

# 根级主测试：后端 go test + golangci-lint，前端 lint/typecheck/关键 Vitest
make test

# 后端
make -C backend test-unit
make -C backend test-integration   # 串行 -p 1
make -C backend test-e2e-local

# 前端
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
```

当前 macOS 本机的 `xcrun` 会把递归 `MAKE` 解析为位于带空格 Xcode-beta 路径中的可执行文件，导致根 Makefile 的 `$(MAKE)` 目标被 shell 错误拆分。若出现 `/Volumes/External: No such file or directory`，使用显式覆盖：

```bash
MAKE=/usr/bin/make /usr/bin/make build
MAKE=/usr/bin/make /usr/bin/make test
```

不要把该本机工具链问题误判为仓库构建失败；CI 和其他设备仍使用标准 `make build` / `make test`。

最低验证要求：

- 仅文档或注释：检查 diff、链接、命令和事实，不要求无关构建。
- 后端局部逻辑：运行受影响 package 的 `go test`；公共接口、repository、gateway 或安全链路扩大到 unit/integration 和 lint。
- migration/Ent：运行相关 repository/integration 测试，重新生成 Ent，并用 `git diff` 确认生成结果完整。
- 前端局部组件：运行目标 Vitest；涉及共享类型、路由、store 或构建配置时增加 `lint:check`、`typecheck` 和必要的 build。
- 跨前后端契约：同时验证后端 handler/service 测试与前端 API/页面测试。
- OpenSpec 变更：额外运行对应 strict validate 和文档列出的专项门禁。
- 上游合并或共享网关热路径：运行架构/lint/integration 检查，并专项回归 fork 的监控、安全审计和协议兼容能力。

不要声称未运行的测试已通过。因 Docker、PostgreSQL、Redis、网络或凭据缺失无法执行时，明确报告未执行项和残余风险。

## Git 与交付边界

- `origin` 是个人 fork，`upstream` 是原项目；fetch、merge、push 前确认目标 remote，不得把上游当作默认推送目标。
- 上游同步必须保留 fork 定制能力，冲突解决后验证共享网关、监控和安全审计路径；不得用上游版本静默覆盖定制实现。
- 未经用户明确要求，不创建提交、不推送、不修改远程历史，也不自动执行上游合并。
- 禁止 force push。不得使用 destructive Git 命令清理用户工作区。
- 提交时使用与仓库一致的 Conventional Commits，且一个提交只包含同一逻辑范围。
- 任务完成前检查 `git status --short`、`git diff --check` 和完整 diff，区分本次修改与用户原有修改。
- 根 `AGENTS.md` 当前被 `.gitignore` 明确忽略，属于本机协作规则；除非用户明确要求改变跟踪策略，不修改 ignore 规则或强制暂存该文件。

## 中央知识库使用规则

本机已核验的中央知识库路径：

```text
/Users/yuan/Desktop/codex-knowledge-base
```

当前项目知识目录：

```text
/Users/yuan/Desktop/codex-knowledge-base/10-projects/sub2api
```

其他设备上的路径可能不同。如果上述路径不存在，先在当前项目同级目录中定位现有的 `codex-knowledge-base`，必要时再全局搜索该目录；不得猜测路径，也不得创建新的中央知识库。定位后以其 `INDEX.yaml` 中 `projects.sub2api.path` 为当前项目知识目录。

### 任务开始前

每次开始处理任务时，先判断本次任务是否需要历史上下文。

出现以下情况时，必须按需读取中央知识库：

- 用户要求继续之前的工作；
- 涉及历史决策、项目约束、设计原因或当前状态；
- 涉及曾经处理过的问题或故障；
- 当前方案可能与既有决策冲突；
- 用户提到“之前”“上次”“继续”“原来的方案”等；
- 当前任务明显依赖跨对话信息。

需要读取时：

1. 读取中央知识库根目录 `AGENTS.md`；
2. 读取中央知识库 `INDEX.yaml`；
3. 从索引确认当前项目目录，默认读取本项目的：
   - `PROJECT.md`
   - `STATUS.md`
   - `CONSTRAINTS.md`
4. 根据任务需要按需读取：
   - `DECISIONS.md`
   - `TODO.md`
   - `GLOSSARY.md`
   - `ARCHITECTURE.md`
   - `INCIDENTS.md`
   - 其他相关文件
5. 不得默认遍历或加载整个中央知识库。

对于完全独立、无需历史信息的简单任务，可以不读取中央知识库。

当前用户说明、当前代码、配置、日志和实际运行结果的优先级高于知识库旧内容。发现冲突时必须明确指出，不得静默覆盖。

### 任务结束后

每次任务完成后，主动检查本次是否产生了值得跨设备、跨对话长期保留的项目知识。

值得沉淀的内容包括：

- 已验证的重要事实；
- 已确认的问题根因；
- 用户明确作出的技术、业务或设计决策；
- 长期有效的约束；
- 对既有知识的明确纠正；
- 会影响后续工作的状态变化；
- 明确且具有后续价值的待办；
- 可跨项目复用的重要经验。

不得沉淀：

- 普通问答；
- 完整聊天记录；
- 临时调试过程；
- 未验证猜测；
- 无长期价值的操作细节；
- 可直接从代码中读取且频繁变化的信息；
- 重复内容；
- 密钥、Token、Cookie、密码或凭据；
- 未脱敏生产日志、客户信息或敏感数据。

如果本次没有产生长期有效知识，不得为了留下记录而创建文件。

### 知识写入方式

需要沉淀时，默认写入中央知识库：

```text
<central-kb>/00-inbox/
```

文件名格式：

```text
YYYY-MM-DD-HHMM-sub2api-<topic>.md
```

内容必须明确区分：

- 已确认事实；
- 新决策；
- 状态变化；
- 待核验；
- 下一步。

Front Matter 至少包含：

```yaml
---
type: session-capture
project: sub2api
date: YYYY-MM-DD
status: pending
source: codex-session
---
```

默认只创建新的独立 Inbox 文件，不直接大范围修改中央知识库正式文件，不覆盖其他会话生成的 Inbox 文件。

凌晨维护任务负责后续的去重、正式归并、更新状态、标记被替代决策、校验，以及正式知识文件和维护报告的 Git 提交与推送。

### 写入前检查

写入前必须：

1. 确认中央知识库路径真实存在；
2. 检查中央知识库当前分支、远程跟踪分支和 `git status --short`；
3. 确认不存在未解决的 Git 冲突；
4. 使用新的独立文件名，并确认目标文件不存在，避免覆盖已有文件；
5. 确认内容不含密钥、Token、Cookie、密码、凭据或未脱敏数据；
6. 默认不提交或推送正式知识文件；只有下述“Inbox 自动提交与推送”规则允许提交本次新建的 Inbox 文件；
7. 发现冲突、归属不明修改或疑似敏感信息时停止写入并提醒用户。

### Inbox 自动提交与推送

当本次任务产生了值得长期保存的知识，并成功写入中央知识库 `00-inbox/` 后，允许自动将本次会话新建的独立 Inbox 文件提交并推送到中央知识库远程仓库，以便其他设备及时同步。

自动提交和推送仅限本次会话新建的 Inbox 文件。不得在此阶段自动提交或推送：

- `10-projects/` 下的正式项目知识文件；
- `HOME.md`；
- `INDEX.yaml`；
- 夜间维护报告；
- 其他会话创建的 Inbox 文件；
- 与本次会话无关的修改；
- 来源不明的文件；
- 敏感信息或未脱敏数据。

执行前必须：

1. 确认中央知识库路径存在并进入该目录；
2. 检查当前分支和远程跟踪分支；
3. 运行 `git status --short`，并检查暂存区文件列表；
4. 确认不存在未解决冲突；
5. 确认本次 Inbox 文件不包含密钥、Token、Cookie、密码、凭据或未脱敏数据；
6. 确认能通过显式 pathspec 只提交本次新建的 Inbox 文件，且暂存区没有会混入本次提交的其他文件。

如果写入前知识库工作区完全干净，则先执行：

```bash
git pull --ff-only
```

然后写入 Inbox 文件。如果写入前已经存在其他未提交的 Inbox 文件，可以保留，但不得覆盖、修改或包含在本次提交中。工作区非干净时不得自动 pull。

出现以下任一情况时，停止自动提交和推送并提醒用户：

- 存在 Git 冲突；
- 无法快进同步；
- 当前分支没有远程跟踪分支；
- 本地与远程分叉；
- 存在无法确认归属的修改；
- 本次 Inbox 疑似包含敏感信息；
- 暂存区已有其他文件，或无法保证只提交本次会话产生的文件。

提交时必须显式指定本次新建的 Inbox 文件，不得使用：

```bash
git add .
git add --all
```

应使用类似：

```bash
git add -- "00-inbox/YYYY-MM-DD-HHMM-sub2api-<topic>.md"
git diff --cached --name-only
git diff --cached --check
git commit -m "docs(kb): capture sub2api session knowledge" -- "00-inbox/YYYY-MM-DD-HHMM-sub2api-<topic>.md"
```

提交后执行普通推送：

```bash
git push
```

永久禁止：

```bash
git push --force
git push --force-with-lease
git reset --hard
git clean -fd
git stash
git rebase
git merge
```

推送成功后检查：

```bash
git status --short
git rev-parse HEAD
git rev-parse @{u}
```

必须确认本次提交已同步至远程跟踪分支，并明确报告未被本次提交包含的既有修改。

若本次没有长期有效知识，不创建 Inbox 文件，不创建提交，也不执行推送。凌晨维护任务仍负责正式知识的去重、归并、校验、提交和推送。
