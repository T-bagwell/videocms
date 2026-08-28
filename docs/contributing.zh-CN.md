# 参与 VideoCMS 贡献

感谢你有兴趣参与贡献！VideoCMS 是一个使用 Go、React 和 PostgreSQL 构建的自托管视频资源管理系统。无论你是发现了 bug、想提出新功能，还是希望帮忙完善文档和翻译，本指南都涵盖了你需要的一切。

## 目录

- [贡献方式](#贡献方式)
- [开发环境搭建](#开发环境搭建)
- [项目结构](#项目结构)
- [仓库约定](#仓库约定)
- [测试](#测试)
- [持续集成](#持续集成)
- [Pull request 流程](#pull-request-流程)
- [文档与本地化](#文档与本地化)
- [常见问题排查](#常见问题排查)
- [获取帮助](#获取帮助)

## 贡献方式

### 报告 bug

- 先搜索[已有 issue](https://github.com/T-bagwell/videocms/issues)，避免重复。
- 提交 issue 时请包含：VideoCMS 版本/commit、操作系统和浏览器、PostgreSQL 版本、复现步骤、预期与实际行为，以及相关日志（扫描/HLS 问题尤其需要后端日志）。

### 提出功能需求

- 请描述你想解决的问题和具体的应用场景，而不仅仅是界面草图。
- 小且聚焦的功能需求更容易讨论和实现。

### 完善文档与翻译

- 语言矩阵和改动文档的规则参见[文档与本地化](#文档与本地化)。

### 提交代码

- 除拼写修正等小改动外，请先开 issue，让维护者在投入时间前给出意见。
- 保持 pull request 小而聚焦（参见 [Pull request 流程](#pull-request-流程)）。

## 开发环境搭建

### 前置条件

- Go —— 版本以 `backend/go.mod` 为准
- Node.js 18+、20+ 或 22+（CI 会跑全部三个版本）
- PostgreSQL 14+
- ffmpeg/ffprobe（转码 MKV/HEVC 需要 libx265）

### 一次性初始化

```bash
# 1. 克隆并进入仓库
git clone git@github.com:T-bagwell/videocms.git
cd videocms

# 2. 创建数据库（幂等）
createdb videocms
# 或者：make db

# 3. （可选）生成示例媒体
make demo

# 4. 启动后端
# macOS 开发机的 Go 环境可能被污染（GOPATH 错误、代理不对）；
# 始终通过仓库自带 wrapper 执行：
./.codex/skills/videocms/scripts/goenv.sh --in backend go run ./cmd/server
# 或者只 source 一次：source ./.codex/skills/videocms/scripts/goenv.sh

# 5. 启动前端开发服务器（/api 代理到 :8080）
cd frontend
npm install
npm run dev
```

打开 http://localhost:5173 ，使用初始管理员 **admin / admin123** 登录。

### 常用命令

| 命令 | 作用 |
| --- | --- |
| `make db` | 创建 `videocms` 数据库（可重复执行） |
| `make server` | 在 http://localhost:8080 运行后端 |
| `make frontend` | 在 :5173 运行 Vite 开发服务器（代理 `/api`） |
| `make demo` | 生成 `demo-media/` 和 `demo-series/` 示例文件 |
| `make build` | 构建后端 bin 与前端 `dist` |
| `make serve` | 单端口生产模式（后端托管 SPA） |

### 环境说明

- 在本仓库中不要裸跑 `go`，请使用
  `./.codex/skills/videocms/scripts/goenv.sh --in backend ...`（模块位于
  `backend/`，所以需要 `--in backend`）。
- macOS 开发机上，后端会自动使用 Homebrew 的 ffmpeg
  （`/usr/local/opt/ffmpeg/bin/`）；系统自带 ffmpeg 在 libx265 上可能崩溃。
- 必须运行 PostgreSQL；迁移会在后端启动时自动应用。

## 项目结构

```
backend/
  cmd/server/     entrypoint
  internal/
    api/          HTTP 处理器与路由
    auth/         JWT + 角色中间件
    media/        扫描、刮削、HLS、流媒体
    db/           pool + SQL 迁移（内嵌）
    models/       领域类型
frontend/
  src/
    pages/        路由组件
    components/   共享 UI
    i18n/         语言 JSON（en/zh/fr/ja/de）
docs/             产品、架构、截图
```

完整的目录映射、API 路由、数据模型、关键流程、安全与扩展点参见
[architecture.md](architecture.md)。

## 仓库约定

### 后端

- 只用 Go 标准库 `net/http` + `pgx/v5`，不使用 Web 框架。
- 改动请先通过 `gofmt` 和 `go vet`。
- 数据库结构变更需要新增编号迁移
  （`backend/internal/db/migrations/NNN_*.sql`）；迁移在启动时自动应用。
  永远不要修改已应用过的迁移，新增一个即可。
- 所有展示视频的列表都必须经过 `visibleEpisodes($N)`
  （定义在 `backend/internal/api/handlers_videos.go`），以尊重每个用户的
  隐藏路径、管理员的标题屏蔽和库屏蔽：首页、继续观看、收藏、播放列表、
  剧集详情和剧集列表都要遵守。
- 管理端内容屏蔽：`blocked_titles` 以不区分大小写的子串匹配标题，并合并进
  `visibleEpisodes`；库级屏蔽位于 `libraries.blocked`，通过 `visiblePaths`
  即使在不 join `libraries` 的子查询中也能生效。
- 媒体端点（`/stream`、`/download`、`/poster`、`/hls/*`）继续接受
  `?token=`，这样 `<video>`/`<img>` 标签无需请求头即可工作。
- API/错误消息保持英文；只有 Web UI 做本地化。

### 前端

- 不允许硬编码 UI 文案——所有用户可见字符串都要走 `useTranslation()`，并
  写入全部五个语言文件
  （`frontend/src/i18n/locales/{en,zh,fr,ja,de}.json`）。
- 切换剧集时播放器绝不能重新挂载 `<video>` 元素：PlayerPage 维护
  `activeId` 状态并调用 `switchEpisode(nextId)`，在同一元素上替换 HLS 源。
- 连续播放：`onEnded` 选择 `queue[idx + 1]` 并切换；不要跳转或重建播放器。

### 剧集与媒体

- 剧集识别位于 `backend/internal/media/episode.go`（`parseEpisode`）；支持的
  标记：`S01E01`、`EP1`、`E01`、`第N集`、`ShowName01Title`、结尾 `(NN)` /
  `  NN`。
- 每个（剧名, 季）组合至少 2 集才分组；每次扫描后由 `rebuildSeries` 重建
  分组。修改解析规则时必须同步更新 `episode_test.go`。
- 剧集列表排序：最新导入的剧集在前（`max(v.created_at)` DESC），然后按
  名称，再按季。

### HLS（很脆弱——不要回退）

- 不要使用 `-hls_playlist_type vod`：否则清单会一直缓冲到整个转码完成，
  长视频无法开始播放。实时增长的清单是有意为之；ffmpeg 进程结束时由服务端
  追加 `#EXT-X-ENDLIST`。
- 不要使用 `-hls_flags temp_file`；保持 `-hls_list_size 0`。
- 用 `expr:gte(t,n_forced*6)` 强制关键帧，得到稳定的 6 秒分片。
- 会话空闲 15 分钟后过期；seek 会从请求的偏移量重新开始转码。

## 测试

```bash
# 后端（始终通过 wrapper）
./.codex/skills/videocms/scripts/goenv.sh --in backend go test ./...
./.codex/skills/videocms/scripts/goenv.sh --in backend go vet ./...

# 前端
cd frontend
npm run lint
npm run test
npm run build
```

- 解析/扫描逻辑要配套单元测试（例如 `internal/media/episode_test.go`）。
- 集成测试（`internal/api/integration_test.go`）在无法连接 PostgreSQL 时
  自动跳过；设置 `TEST_PG_DSN` 可运行它们。
- 依赖网络的刮削测试在未设置 `NETWORK_TEST=1` 时跳过。

## 持续集成

GitHub Actions 在推送到 `main` 和 pull request 时运行两个 workflow：

| Workflow | 文件 | 运行内容 |
| --- | --- | --- |
| Backend CI | `.github/workflows/backend.yml` | 在 `backend/` 执行 `go build`、`go vet`、golangci-lint、`go test`（单元 + PostgreSQL 集成测试） |
| Frontend CI | `.github/workflows/webpack.yml` | 在 `frontend/` 执行 `npm ci`、ESLint、Vitest、`npm run build`（Node 18/20/22） |
| CodeQL | `.github/workflows/codeql.yml` | Go 与 JavaScript 安全扫描（push、PR、每周） |
| Release | `.github/workflows/release.yml` | 打 `v*` tag 时构建跨平台二进制并发布 GitHub Release |

请求 review 前请确保两者都为绿色。

## Pull request 流程

1. 非平凡改动先开 issue，讨论方案。
2. 从 `main` 拉分支，起一个简短描述性名称（`fix/`、`feat/`、`docs/`、
   `refactor/` 等）。
3. 提交保持聚焦；一个 PR 只做一项逻辑改动。
4. 打开 PR 前：
   - `gofmt`、`go vet`、`golangci-lint run`、`go test ./...` 通过
   - `npm run lint`、`npm run test`、`npm run build` 通过
   - UI 改动在 PR 描述里附带截图
5. 在 PR 中说明改了什么、为什么改，并引用其修复的 issue（`Closes #123`）。
6. 用户可见的改动更新 [changelog.md](changelog.md)。
7. 改动文档时，同步更新所有已有语言（见下文）。

## 文档与本地化

仓库维护多套文档；改动任一文档时，请同步更新所有已有语言，并保持代码块
成对闭合。

| 文档集 | 语言 |
| --- | --- |
| README | en、zh-CN、ja |
| 产品文档（`docs/product.*.md`） | en、zh-CN、fr、ja、de |
| 架构文档（`docs/architecture.*.md`） | en、zh-CN、ja |
| 贡献指南 | en、zh-CN、ja |

- 新增或重命名文档文件时，同步更新 `INDEX.md` 索引。
- Web UI 支持五种语言；默认是英文，缺失的 key 回退到英文。

### 为 UI 增加语言

1. 添加 `frontend/src/i18n/locales/<code>.json`（复制英文文件并翻译）。
2. 在 `frontend/src/i18n/index.js` 中注册（`SUPPORTED_LANGS` + `resources`）。
3. 保持 key 结构完全一致，以便回退到英文。
4. 更新列出支持语言的 README 和文档。

## 常见问题排查

- 在 `backend/` 之外执行 `go run`/`go build` 会失败——始终使用
  `--in backend`。
- 崩溃后遗留为 `scanning` 状态的扫描会在启动时重置为 `error`；从管理页
  重新扫描即可。
- 扫描器会跳过 macOS 的 `._*` 文件和 `.m3u8` 流目录——请保持该行为。
- `make serve` 绑定 :8080 并托管 SPA；开发时同时使用 `make server` 和
  `make frontend`（Vite 把 `/api` 代理到 :8080）。

## 获取帮助

- GitHub [issues](https://github.com/T-bagwell/videocms/issues) 用于提交 bug
  和功能需求。
- 仓库附带项目级 Codex skill（`.codex/skills/videocms/`），其中编码了上述
  环境、命令和约定；在本仓库工作的 Codex 智能体应加载它。
