# 🎬 VideoCMS

> **自托管的视频资源管理系统** — Go · React · PostgreSQL

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-14-4169E1?logo=postgresql&logoColor=white)
![i18n](https://img.shields.io/badge/i18n-5%20languages-8A2BE2)

**语言:** [English](README.md) · 中文 · [日本語](README.ja.md)

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
| ▶️ 播放 | H.264/WebM 原生播放（HTTP Range）；**MKV/HEVC 实时 HLS 转码**；字幕自动识别（SRT→WebVTT）；支持下载 |
| 👤 个性化 | 继续观看、收藏（视频与剧集）、可顺序播放的播放列表 |
| 🔐 用户 | 注册/登录（JWT）；管理员/普通用户角色；带安全守卫的用户管理 |
| 🚫 内容屏蔽 | 管理员可在后台按媒资名屏蔽——对所有人隐藏，文件和记录保留，可随时解除 |
| 🚫 路径过滤 | 按用户隐藏任意服务器路径——首页、剧集、收藏、继续观看、播放列表全部生效 |
| 🌐 界面 | i18n：**English（默认）、中文、Français、日本語、Deutsch** |

## 截图

> *即将补充——`make serve` 后访问 `http://<服务器IP>:8080` 即可看到界面。*

## 文档

所有文档均为多语言版本，从 **[文档索引](docs/README.md)** 开始：

| 文档 | 语言 | 读者 |
| --- | --- | --- |
| [产品文档](docs/product.zh-CN.md) | EN · 中文 · FR · JA · DE | 终端用户 |
| [系统架构](docs/architecture.zh-CN.md) | EN · 中文 · JA | 开发者 |
| [README](README.md) / [日本語](README.ja.md) | English · 日本語 | 所有人 |

## 快速开始

### 运行环境

- Go 1.22+（构建，或使用预编译二进制）
- PostgreSQL 14+
- ffmpeg + ffprobe（元数据、海报、转码）
- Node.js 18+（仅前端开发；生产界面由后端托管）

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

用初始管理员 **admin / admin123** 登录后立即修改密码（管理 → 用户管理 → 重置密码）。
然后在 管理 → 媒体库 → 扫描 添加第一个媒体库。

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
| `TMDB_API_KEY` / `TMDB_LANGUAGE` | 空 / zh-CN | 元数据刮削 |
| `SCAN_WORKERS` | `4` | 并行扫描工作数（1-16） |
| `WEB_ROOT` | 自动（`frontend/dist`） | 生产模式的前端目录 |

## 项目结构

```
backend/                 Go 服务（net/http + pgx）
  cmd/server/            入口
  internal/api/          HTTP 处理器、路由、中间件
  internal/auth/         JWT + 角色中间件
  internal/media/        扫描器、TMDB 刮削、HLS 管理、流媒体
  internal/db/           连接池 + 内嵌 SQL 迁移
  internal/models/       领域模型
frontend/                React 18 SPA（Vite）
  src/i18n/locales/      en / zh / fr / ja / de
  src/pages/             浏览、播放、剧集、播放列表、管理…
docs/                    产品 + 架构文档（多语言）
scripts/                 演示素材生成器
```

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go（net/http、pgx/v5）、JWT（HS256）、bcrypt |
| 前端 | React 18、Vite、react-router、i18next、hls.js |
| 数据库 | PostgreSQL 14（内嵌 SQL 迁移） |
| 媒体 | ffprobe（元数据）、ffmpeg（海报、HLS 转码） |
| 文档 | Markdown + Mermaid（GitHub 渲染） |

## 安全

- 所有变更操作仅限管理员；媒体 URL 需要用户 JWT（请求头或 `?token=`）
- 密码 bcrypt 哈希；角色每次请求从数据库重新加载
- HLS 分片名校验并限定在会话目录内
- SQL 全程参数化
- **生产环境**：设置强 `JWT_SECRET`、前置 HTTPS 反向代理、
  用 `ADMIN_USERNAME/ADMIN_PASSWORD` 指定初始账号

另见 [SECURITY.md](SECURITY.md)。

## 路线图

- [x] 媒体库扫描（并行、可取消、实时进度）
- [x] 元数据 + 海报 + TMDB 刮削
- [x] 原生播放 + HLS 转码
- [x] 剧集自动归组（多种命名规则）
- [x] 收藏（视频与剧集）、播放列表、继续观看
- [x] i18n（en/zh/fr/ja/de）
- [ ] 文件系统监听增量入库
- [ ] 自适应码率（多档 HLS）
- [ ] 内嵌字幕提取 / 上传
- [ ] 签名短时 URL 公开分享

## 贡献

见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## 许可证

[Apache License 2.0](LICENSE)
