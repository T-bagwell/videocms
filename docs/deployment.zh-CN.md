# 部署

VideoCMS 支持两种拓扑：

1. **单服务（默认）** — 后端在同一个端口上同时托管构建好的 React 前端与
   REST API（`make serve`，或后端二进制旁边存在 `frontend/dist`）。
2. **前后端分离** — 后端作为纯 API 服务运行，前端作为静态文件部署在
   nginx 或任意 Web 服务器上。界面的所有操作都通过 RESTful API 暴露，
   因此也可以完全用程序化方式驱动系统。

## 后端作为纯 API 服务

启动后端二进制时不要设置 `WEB_ROOT`，且旁边不要有 `frontend/dist` 目录——
此时只会挂载 `/api` 路由。

```bash
export PORT=8080
export DATABASE_URL=postgres://videocms:videocms@localhost:5432/videocms?sslmode=disable
export JWT_SECRET="$(openssl rand -hex 32)"
export CORS_ORIGINS=https://media.example.com   # 可选；默认 *
./videocms-server
```

健康检查：`GET /api/healthz` → `{"status":"ok"}`。

其他变量：`ADMIN_USERNAME`/`ADMIN_PASSWORD`、`DATA_DIR`、`FFMPEG_BIN`/
`FFPROBE_BIN`、`YTDLP_PATH`、`TMDB_API_KEY`、`SCAN_WORKERS`、
`WATCH_INTERVAL`（完整配置表见 README）。

## 前端作为静态文件

```bash
cd frontend
npm ci
npm run build        # 输出 frontend/dist
```

通过下面两种方式之一让前端指向 API：

- **同源反向代理** — 托管 `frontend/dist`，并把 `/api` 反代到后端
  （见 [deploy/nginx.conf.example](../deploy/nginx.conf.example)）。无需额外配置。
- **跨域** — 构建时使用 `VITE_API_BASE_URL=https://api.example.com`，
  或在应用启动前在运行时注入基地址：

  ```html
  <script>
    window.__VIDEOCMS_API_BASE__ = 'https://api.example.com';
  </script>
  ```

  运行时注入优先于构建期变量。当前后端不在同一源时，在后端设置
  `CORS_ORIGINS` 为前端来源（留空则允许任意来源；请求使用 Bearer Token
  认证，不依赖 Cookie）。

## 使用 REST API

所有端点位于 `/api` 下并返回 JSON。错误使用
`{"error": "message"}` 与相应状态码。

1. **认证**：

   ```bash
   curl -s -X POST https://api.example.com/api/auth/login \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"admin123"}'
   # → {"token":"<jwt>","user":{...}}
   ```

2. **携带 Bearer Token 调用 API**：

   ```bash
   curl -s https://api.example.com/api/libraries \
     -H 'Authorization: Bearer <jwt>'
   ```

   媒体端点（`/stream`、`/download`、`/poster`、`/hls/*`、
   `/subtitles/*`）也支持把 token 作为查询参数（`?token=<jwt>`），
   以便 `<video>`/`<img>` 标签无需请求头即可工作。

常用端点（管理端点需要管理员账号）：

| 方法 | 路径 | 权限 | 用途 |
| --- | --- | --- | --- |
| POST | `/api/auth/login` | 公开 | 获取 JWT |
| GET | `/api/auth/sso` | 公开 | 已配置哪些 SSO 提供方 |
| GET | `/api/auth/oidc/start` | 公开 | 开始 OIDC 登录（重定向到 IdP） |
| GET | `/api/auth/saml/login` / `POST /api/auth/saml/acs` | 公开 | SAML 登录流程 |
| GET | `/api/auth/saml/metadata` | 公开 | 供 IdP 配置的 SP 元数据 |
| GET | `/api/libraries` | 用户 | 媒体库列表 |
| POST | `/api/libraries` | 管理员 | 添加媒体库（服务器绝对路径） |
| POST | `/api/libraries/{id}/scan` | 管理员 | 开始扫描 |
| POST | `/api/libraries/{id}/health` | 管理员 | 健康检查（缺失/损坏/重复） |
| POST | `/api/libraries/{id}/health/keep-best` | 管理员 | 保留最佳版本，其余移入回收站 |
| POST | `/api/libraries/{id}/export-nfo` / `import-nfo` | 管理员 | Kodi 风格 NFO 导出/导入 |
| GET | `/api/videos` | 用户 | 搜索/浏览视频 |
| GET | `/api/videos/{id}` | 用户 | 视频详情 |
| GET | `/api/videos/{id}/stream` | 用户 | HTTP Range 流媒体 |
| GET | `/api/videos/{id}/download` | 用户 | 下载原文件 |
| GET | `/api/videos/{id}/download/remux` | 用户 | 可选轨道的 MP4/MKV |
| GET | `/api/videos/{id}/tracks` | 用户 | 音轨/字幕列表 |
| GET/PUT/DELETE | `/api/videos/{id}/skip-interval(s)` | 用户 | 片头/片尾跳过区间 |
| POST | `/api/videos/{id}/transcribe` | 管理员 | Whisper 语音转写 |
| GET/POST/DELETE | `/api/videos/{id}/tags` | 用户 | 视频标签 |
| POST | `/api/videos/{id}/analyze` | 管理员 | 运行 AI 打标 |
| GET/POST | `/api/videos/{id}/comments`、`PUT …/rating` | 用户 | 评论与评分 |
| GET | `/api/videos/{id}/similar` | 用户 | 相似视频推荐 |
| POST | `/api/uploads` | 管理员 | 创建分片上传会话 |
| PUT | `/api/uploads/{id}/chunk/{index}` | 管理员 | 上传一个分片 |
| POST | `/api/uploads/{id}/complete` | 管理员 | 完成上传 |
| POST | `/api/downloads` | 管理员 | 加入 yt-dlp 下载 |
| GET | `/api/downloads` | 管理员 | 下载任务列表 |
| GET/PUT | `/api/watch/rooms/{id}` | 用户 | 一起看会话状态 |
| GET/POST | `/api/live`、`/api/live/{id}/chat` | 用户/管理员 | 直播与聊天 |
| GET/POST | `/api/admin/blocked-titles` | 管理员 | 标题屏蔽管理 |
| GET | `/api/admin/users` | 管理员 | 用户管理 |
| POST | `/api/admin/videos/batch` | 管理员 | 批量打标/清标/移入回收站 |
| GET | `/api/admin/trash`、`POST …/restore` | 管理员 | 回收站 |
| GET/POST/PATCH/DELETE | `/api/admin/storage-pools` | 管理员 | 本地/S3/SFTP 存储池 |
| GET | `/api/admin/jobs` / `system` | 管理员 | 任务看板 + 磁盘用量 |
| POST | `/api/admin/maintenance/run` | 管理员 | 立即执行维护 |
| GET | `/api/admin/backups[/{name}]` | 管理员 | 列出/下载备份 |
| GET/POST/PATCH/DELETE | `/api/admin/webhooks` | 管理员 | 带签名的 Webhook 订阅 |
| POST | `/api/admin/notify/test` | 管理员 | 发送测试通知 |
| GET | `/api/openapi.json` | 公开 | API 的 OpenAPI 描述 |
| GET | `/api/healthz` | 公开 | 健康检查 |

完整路由表与数据流见 [architecture.zh-CN.md](architecture.zh-CN.md)。

## Docker 与 Kubernetes

官方容器镜像由仓库根目录 `Dockerfile` 构建（多阶段：Node 22 构建前端、Go 构建后端，
`alpine` 运行镜像内置 ffmpeg、SPA 与 API，端口 8080）。`docker-compose.yml`
一键运行 PostgreSQL 16 + VideoCMS，使用两个数据卷（数据库与媒体数据）：

```bash
JWT_SECRET="$(openssl rand -hex 32)" docker compose up -d --build
```

Kubernetes 使用 `videocms-helm/` 目录下的 Helm chart：

```bash
helm install videocms ./videocms-helm \
  --set env.JWT_SECRET="$(openssl rand -hex 32)" \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=media.example.com
```

Chart 支持水平扩展（`autoscaling.enabled=true`、`replicaCount>1`），需要共享媒体卷
（使用 NFS/SMB/CephFS 等 `ReadWriteMany` 存储类）。转码会话与文件监视器按副本独立运行：
多副本部署时应将 `DATA_DIR` 指向共享存储，并只在一个副本上执行定时扫描，避免重复工作。

## 可选集成

### DLNA / Chromecast

把媒体库开放给局域网内 UPnP/DLNA 电视与播放器：

```bash
export DLNA_ENABLED=1
export DLNA_FRIENDLY_NAME="Home Media"        # 可选
export DLNA_ALLOWED_IPS="192.168.3.0/24"      # 可选；留空 = 整个局域网
```

服务器在 UDP 1900 应答 SSDP，并提供 `/dlna/device.xml`、
`/dlna/content/{id}`（DIDL-Lite）与 `/dlna/video/{id}/stream`。支持 Cast 的
浏览器中，播放器会显示「投屏到电视」（Chromecast）按钮；投屏走短期分享链接，
因此 Chromecast 需要能访问服务器。

### SAML 2.0 单点登录

先生成 SP 密钥对（CN 用你对外暴露的域名），再把后端指向 IdP 元数据：

```bash
openssl req -x509 -newkey rsa:2048 -keyout sp.key -out sp.crt \
  -days 3650 -nodes -subj "/CN=videocms"
export SAML_IDP_METADATA_URL=https://idp.example.com/metadata
export SAML_SP_CERT=/etc/videocms/sp.crt
export SAML_SP_KEY=/etc/videocms/sp.key
export SAML_ACS_URL=https://media.example.com/api/auth/saml/acs
export SAML_SP_ENTITY_ID=https://media.example.com/api/auth/saml/acs
```

取回 `https://media.example.com/api/auth/saml/metadata` 并在 IdP 中注册。
用户绑定到带 `saml:` 前缀的 `users.oauth_sub`；`roles` 属性包含 "admin"
时首次登录即授予管理员。

### 邮件通知（SMTP）

```bash
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587                 # 465 = 隐式 TLS
export SMTP_USER=videocms@example.com
export SMTP_PASSWORD='secret'
export NOTIFY_EMAIL_FROM=videocms@example.com
export NOTIFY_EMAIL_TO=ops@example.com,admin@example.com
```

扫描、上传与下载事件会以纯文本邮件投递（587/25 走 STARTTLS，465 走隐式
TLS）。可用管理概览页按钮或 `POST /api/admin/notify/test` 测试。
