# 🎬 VideoCMS

> **自托管的视频资源管理系统** — Go · React · PostgreSQL

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](../LICENSE)
![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14-4169E1?logo=postgresql&logoColor=white)
![i18n](https://img.shields.io/badge/i18n-5%20languages-8A2BE2)

**语言:** [English](../README.md) · 中文 · [日本語](README.ja.md)

把服务器磁盘上的文件夹变成可浏览、可搜索的视频媒体库。扫描一次，所有视频就带上海报、
元数据、观看进度、收藏与播放列表——带序号的文件还会自动归组为剧集。

---

## 目录

- [功能特性](#功能特性)
- [截图](#截图)
- [文档](#文档)
- [快速开始](#快速开始)
- [局域网 / 手机访问](#局域网--手机访问)
- [配置](#配置)
- [项目结构](#项目结构)
- [技术栈](#技术栈)
- [安全](#安全)
- [路线图](#路线图)
- [贡献](#贡献)
- [许可证](#许可证)

## 功能特性

| 领域 | 亮点 |
| --- | --- |
| 📂 媒体库 | 任意服务器文件夹；支持手输路径或内置**服务器目录选择器** |
| 🔍 扫描 | 递归发现 mp4/mkv/webm/avi/mov/ts…；并行探测（4 worker，`SCAN_WORKERS` 可调）；实时进度；**可随时取消**；自动跳过 macOS `._` 文件和 `.m3u8` 流文件夹 |
| 🏷️ 元数据 | ffprobe 提取编码/分辨率/时长；自动生成海报；可编辑标题/年份/简介/类型；可选 **TMDB 在线刮削** |
| 📺 剧集 | 带序号的文件（`S01E01`、`EP1`、`第1集`、`剧名01集名`…）自动按集数归组；按季区分；支持列表连播 |
| ▶️ 播放 | H.264/WebM 原生播放（HTTP Range）；**MKV/HEVC 实时转码为多档自适应 HLS**（可切换清晰度）；字幕自动识别（SRT→WebVTT）、内嵌字幕提取、字幕上传、**多语言切换**与按用户字幕偏好；**下载为 MKV/MP4，可选音轨与字幕（转封装、不重编码）** |
| ⬆️ 上传与下载 | 管理后台**上传**页：分片、可续传地上传到任意服务器文件夹（位于媒体库内自动收录）；**yt-dlp** 下载队列，支持定时重复 |
| 🔗 分享 | 支持**视频、剧集与播放列表**的短时公开分享链接（签名、限期、可撤销、可选密码与域名白名单）——拿到链接即可免登录观看；同样遵循内容屏蔽 |
| 👤 个性化 | 继续观看、收藏（视频与剧集）、可顺序播放的播放列表 |
| 🔐 用户 | 注册/登录（JWT）；管理员/普通用户角色；带安全守卫的用户管理 |
| 🚫 内容屏蔽 | 管理员可在后台按媒资名屏蔽——对所有人隐藏，文件和记录保留，可随时解除 |
| 🚫 媒体库屏蔽 | 管理后台一键屏蔽整个媒体库——其媒资对所有人隐藏，不删除任何内容 |
| 🚫 路径过滤 | 按用户隐藏任意服务器路径——首页、剧集、收藏、继续观看、播放列表全部生效 |
| 🌐 界面 | i18n：**English（默认）、中文、Français、日本語、Deutsch** |

## 内容管控

三层独立的内容可见性控制，互不影响——任何一层都不会删除文件或记录：

| 层级 | 谁管理 | 作用范围 | 生效范围 |
| --- | --- | --- | --- |
| 🏷️ 标题屏蔽 | 管理员 | 标题包含所填文本的媒资 | 所有列表、所有人 |
| 📚 媒体库屏蔽 | 管理员 | 整个媒体库（其中的全部媒资） | 所有列表、所有人 |
| 🛤️ 路径过滤 | 每个用户 | 用户自行选择的任意服务器路径 | 所有列表、仅该用户 |

三层都在每次列表查询时于 SQL 中求值（首页、剧集、收藏、继续观看、播放列表），
被屏蔽的内容会从所有位置同时消失，解除后立即恢复。

## 截图

![首页](screenshots/home.png)
![剧集](screenshots/series.png)
![视频详情](screenshots/detail.png)
![播放器](screenshots/player.png)

## 文档

所有文档均为多语言版本，从 **[文档索引](INDEX.md)** 开始：

| 文档 | 语言 | 读者 |
| --- | --- | --- |
| [产品文档](product.zh-CN.md) | EN · 中文 · FR · JA · DE | 终端用户 |
| [系统架构](architecture.zh-CN.md) | EN · 中文 · JA | 开发者 |
| [部署指南](deployment.zh-CN.md) | EN · 中文 · JA | 运维人员 |
| [English](../README.md) / [中文](README.zh-CN.md) / [日本語](README.ja.md) | English · 中文 · 日本語 | 所有人 |

## 快速开始

### 运行环境

- Go 1.26+（构建，或使用预编译二进制）
- PostgreSQL 14+
- ffmpeg + ffprobe（元数据、海报、转码）
- yt-dlp（可选——管理后台「下载」队列依赖它）
- Node.js 20+（仅前端开发；生产界面由后端托管）

### 安装

```bash
# 1. 数据库
createdb videocms                          # 或：docker compose up -d db

# 2.（可选）生成演示视频
./scripts/make-demo-media.sh

# 3. 后端——首次启动自动建表 + admin/admin123
cd backend && go run ./cmd/server

# 4. 前端（开发模式，热更新）
cd frontend && npm install && npm run dev  # http://localhost:5173
```

生产式单端口部署：

```bash
make serve                                 # 构建前端并统一在 :8080 提供服务
```

如需把后端作为纯 API 服务、前端单独部署（nginx 或任意静态服务器），
参见 [deployment.zh-CN.md](deployment.zh-CN.md)。

用初始管理员 **admin / admin123** 登录后立即修改密码（管理 → 用户管理 → 重置密码）。
然后在 管理 → 媒体库 → 扫描 添加第一个媒体库（路径必须是服务器绝对路径，
例如 `/media/movies`）。

也可以从 **管理 → 上传** 直接把文件分片上传到服务器文件夹（位于媒体库内会自动收录），
或用 **管理 → 下载** 通过 yt-dlp 抓取网络视频（支持定时重复）。

## 局域网 / 手机访问

1. 查询服务器 IP：`ipconfig getifaddr en0`（如 `192.168.3.19`）
2. 手机连接**同一网络** → 打开 `http://192.168.3.19:8080`
3. 首次运行若 macOS 弹防火墙提示，允许即可

> 明文 HTTP + 开发 JWT 仅建议在可信局域网使用；公网访问请参考[安全](#安全)。

## 配置

全部通过环境变量配置：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `PORT` | `8080` | 监听地址 |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | 数据库连接串 |
| `JWT_SECRET` | 开发用常量 | 令牌签名密钥——**生产必须设置强密钥** |
| `DATA_DIR` | `data` | 海报 + HLS 分片 |
| `ADMIN_USERNAME` / `ADMIN_PASSWORD` | admin / admin123 | 初始管理员 |
| `FFPROBE_BIN` / `FFMPEG_BIN` | 自动探测 | 工具路径（含 Homebrew 回退） |
| `TMDB_API_KEY` / `TMDB_LANGUAGE` | 空 / zh-CN | 元数据刮削；未配置 key 时自动使用免费的 TVMaze、AniList 与 Wikipedia |
| `TVMAZE_ENABLED` | `1` | 设为 `0` 可关闭免密钥的 TVMaze 兜底刮削 |
| `ANILIST_ENABLED` | `1` | 设为 `0` 可关闭免密钥的 AniList 兜底刮削 |
| `WIKIPEDIA_LANG` / `WIKIPEDIA_ENABLED` | `en` / `1` | 免密钥 Wikipedia 兜底的语言版本与开关 |
| `SCAN_WORKERS` | `4` | 并行扫描工作数（1-16） |
| `WATCH_INTERVAL` | `30` | 增量扫描的兜底间隔（fsnotify 事件即时索引）；`0` 关闭监听 |
| `HLS_HW_ACCEL` | 空（软件 x264） | HLS 视频编码器：`videotoolbox`、`nvenc`、`qsv` 或 `vaapi`；留空用 libx264 |
| `HLS_VAAPI_DEVICE` | `/dev/dri/renderD128` | VAAPI 渲染设备（配合 `HLS_HW_ACCEL=vaapi` 使用） |
| `HLS_TONE_MAP` | `0` | 设为 `1` 在 HLS 转码中启用 HDR→SDR 色调映射 |
| `SUBTITLE_OS_USERNAME` / `SUBTITLE_OS_PASSWORD` / `SUBTITLE_OS_API_KEY` | 空 | 在线字幕搜索用的 OpenSubtitles 凭证 |
| `RTMP_INGEST_URL` | `rtmp://localhost:1935/live` | RTMP 推流基础地址（nginx-rtmp 或等价服务）；直播会附加自己的 key |
| `WHISPER_BIN` / `WHISPER_MODEL` | 空 | whisper.cpp 可执行文件与模型路径（语音转写用） |
| `SCRAPE_CUSTOM_URL` | 空 | 自定义 JSON 刮削端点；`%s` 会被替换为 URL 转义后的标题 |
| `AI_TAG_BIN` | 空 | 外部 AI 打标工具；接收媒体路径，每行输出一个标签 |
| `YTDLP_PATH` | PATH 上的 `yt-dlp` | 「下载」队列使用的 yt-dlp 二进制 |
| `WEB_ROOT` | 自动（`frontend/dist`） | 单服务模式托管的前端目录；不设置即纯 API 部署 |
| `CORS_ORIGINS` | 空（`*`） | 允许调用 API 的浏览器来源（逗号分隔，用于前后端分离部署） |
| `VITE_API_BASE_URL` | 空 | 前端构建期 API 基地址（跨域部署用；运行时可用 `window.__VIDEOCMS_API_BASE__` 覆盖） |

## 项目结构

```
backend/                 Go 服务（net/http + pgx）
  cmd/server/            入口
  internal/api/          HTTP 处理器、路由、中间件
  internal/auth/         JWT + 角色中间件
  internal/media/        扫描器、TMDB 刮削、HLS 管理、流媒体
  internal/db/           连接池 + 内嵌 SQL 迁移
  internal/models/       领域模型
frontend/                React 19 SPA（Vite）
  src/i18n/locales/      en / zh / fr / ja / de
  src/pages/             浏览、播放、剧集、播放列表、管理…
docs/                    全部文档（多语言）
scripts/                 演示素材生成器
```

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go（net/http、pgx/v5）、JWT（HS256）、bcrypt |
| 前端 | React 19、Vite 8、react-router、i18next、hls.js |
| 数据库 | PostgreSQL 14（内嵌 SQL 迁移） |
| 媒体 | ffprobe（元数据）、ffmpeg（海报、HLS 转码） |
| 文档 | Markdown + Mermaid（GitHub 渲染） |
| 质量 | ESLint 10、Vitest 4、golangci-lint（CI 中的 lint 与单元测试） |

## 安全

- 所有变更操作仅限管理员；媒体 URL 需要用户 JWT（请求头或 `?token=`）
- 密码 bcrypt 哈希；角色每次请求从数据库重新加载
- HLS 分片名校验并限定在会话目录内
- SQL 全程参数化
- **生产环境**：设置强 `JWT_SECRET`、前置 HTTPS 反向代理、
  用 `ADMIN_USERNAME/ADMIN_PASSWORD` 指定初始账号

另见 [security.md](security.md)。

## 路线图

计划中的能力参考了同类自托管视频项目的功能集
（Jellyfin、MediaCMS、Stash、Kirari04/videocms、yt-dlp 工具等）。

### 已完成

- [x] 媒体库扫描（并行、可取消、实时进度）+ 事件驱动文件监听
- [x] 元数据 + 海报：TMDB 刮削，无密钥时 TVMaze 兜底
- [x] 原生播放 + 自适应码率 HLS 转码
- [x] 剧集自动归组（S01E01、EP1、第N集、纯序号文件名…）
- [x] 收藏（视频与剧集）、播放列表、继续观看
- [x] 内容管控：标题屏蔽、媒体库屏蔽、按用户路径过滤
- [x] 字幕：内嵌提取、上传、多语言字幕轨、用户偏好
- [x] 公开分享：视频/剧集/播放列表的签名短链，支持密码与域名白名单
- [x] i18n（en/zh/fr/ja/de）
- [x] 管理端：数据导出/备份、打开服务器文件夹、目录选择器

### 计划中

**上传与下载**

- [x] 浏览器分片、可续传上传，带队列的上传管理器
- [x] 下载为 MP4/MKV，可勾选音轨/字幕轨（无需重编码）
- [x] 集成 yt-dlp：定时抓取在线网站视频/频道入库

**播放与字幕**

- [x] 多音轨播放与切换（独立 HLS 音轨）
- [x] ASS 样式软字幕
- [x] 字幕同步/偏移调整（直接播放）
- [x] 自动字幕下载与匹配
- [x] 硬件加速转码（VAAPI/NVENC/QSV）与 HDR 色调映射
- [x] 预览时间轴缩略图
- [ ] 片头/片尾跳过
- [x] 一起看（同步播放会话）
- [x] 投屏（Web AirPlay）
- [ ] 投屏（Chromecast / DLNA）
- [x] RTMP 直播推流入库，内置聊天

**元数据与 AI**

- [x] 本地语音转写（Whisper）→ 可搜索的字幕/文稿
- [x] 可插拔元数据源 / 自定义刮削器，支持单条覆盖
- [x] AI 打标、场景识别与图像分析，辅助搜索
- [x] 媒体健康检查：重复检测、损坏文件检查、保留最佳版本清理
- [x] 相似视频推荐与标签云

**整理与搜索**

- [ ] 用户自定义标签、智能合集、保存的筛选条件
- [ ] 标题/简介/文稿/标签的全文与模糊搜索
- [ ] 批量整理（移动、重命名、批量打标）+ 回收站
- [ ] NFO 元数据导入/导出，兼容 Plex/Jellyfin/Kodi

**用户、分享与社交**

- [ ] 评论、评分与动态流
- [ ] OIDC/SAML 单点登录
- [ ] 家长控制（PIN/分级）与用户配额
- [ ] 分享页自定义与播放器嵌入
- [ ] 通知（邮件/Webhook/Apprise）：扫描、上传、转码事件

**存储与运维**

- [ ] 存储池：本地、S3 兼容、SFTP，管理端路由
- [ ] 后台任务看板（监控/取消/重试）+ 更丰富的系统统计
- [ ] 定时维护：重扫、健康检查、元数据备份/恢复
- [ ] Webhook + 完善的公开 REST API，便于第三方自动化
- [ ] PWA 离线下载与移动端体验打磨

## 贡献

欢迎参与贡献！参见贡献指南：

- 中文：[contributing.zh-CN.md](contributing.zh-CN.md)
- English：[contributing.md](contributing.md)
- 日本語：[contributing.ja.md](contributing.ja.md)

## 许可证

[Apache License 2.0](../LICENSE)
