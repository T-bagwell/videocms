# VideoCMS — 视频资源管理系统

> **语言:** [English](README.md) | 中文 | [日本語](README.ja.md)

一个自托管的视频资源管理系统：**Go 后端 + PostgreSQL 数据库 + React 前端**。
把服务器硬盘上的视频文件夹「扫」进媒体库，自动提取元数据（标题、年份、分辨率、编码、时长）、生成海报，
支持网页播放、观看进度、收藏与播放列表，界面支持多语言。

## 架构

```
┌─────────────────┐   HTTP/JSON + Range 流媒体   ┌──────────────────┐
│  React 前端      │ ───────────────────────────▶ │  Go 后端           │
│  (i18n: en/zh/   │                              │  (net/http, 8080) │
│   fr/ja/de)      │                              └───────┬──────────┘
└─────────────────┘                                       │ ffprobe/ffmpeg
        ▲                                                 ▼
        │ 代理 /api                            媒体库磁盘文件夹（视频文件）
        └───────────────────────────────────────────────┘
                                            │
   PostgreSQL ── 元数据 / 进度 / 收藏 / 播放列表
```

## 功能

- 媒体库管理：添加/删除磁盘路径，后台扫描（递归查找 mp4/mkv/webm/avi/mov 等）
- 扫描性能：并行探测（默认 4 个 worker，可用 `SCAN_WORKERS` 调整）、实时进度、
  可随时取消；自动跳过 macOS `._` 假文件和 `.m3u8` HLS 流文件夹；取消/重启不会误标已收录视频
- 元数据：ffprobe 提取时长/分辨率/编码，文件名解析标题与年份，ffmpeg 抽取关键帧海报，
  可选 **TMDB 在线刮削**（中文标题/简介/类型/海报）
- 播放：HTTP Range 断点流媒体 + **HLS 转码播放**（MKV/HEVC 等格式自动转码，支持续播和跳转）；
  字幕自动识别（SRT 在线转 WebVTT）
- 用户系统：注册/登录、JWT（7 天），管理员/普通用户角色；管理员可管理用户
- 社交功能：收藏、播放列表（顺序播放）、继续观看
- 管理后台：统计概览、媒体库扫描（含服务端目录选择器）、视频元数据编辑、海报上传、用户管理
- **多语言界面**：默认英文，可切换 中文 / English / Français / 日本語 / Deutsch

## 快速开始

```bash
# 1. 数据库
createdb videocms                       # 或用 docker compose up -d db

# 2.（可选）生成演示视频
./scripts/make-demo-media.sh

# 3. 后端（首次启动自动建表 + 创建 admin/admin123）
cd backend && go run ./cmd/server

# 4. 前端
cd frontend && npm install && npm run dev   # http://localhost:5173

# 5. 手机/局域网（前端打包进 Go 服务，一个端口）
make serve                                  # 访问 http://<局域网IP>:8080
```

环境变量与 API 一览见 [README.md](README.md)（字段一致，仅语言不同）。

## 已知限制

- 浏览器直接播放 H.264 MP4 / WebM；MKV、HEVC 走单码率 HLS 转码（空闲 15 分钟回收），
  伪装格式可在播放器点「转码播放」
- TMDB 刮削需要能访问 api.themoviedb.org
- 扫描为增量全量重扫，可扩展为文件系统监听
