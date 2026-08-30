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
| GET | `/api/libraries` | 用户 | 媒体库列表 |
| POST | `/api/libraries` | 管理员 | 添加媒体库（服务器绝对路径） |
| POST | `/api/libraries/{id}/scan` | 管理员 | 开始扫描 |
| GET | `/api/videos` | 用户 | 搜索/浏览视频 |
| GET | `/api/videos/{id}` | 用户 | 视频详情 |
| GET | `/api/videos/{id}/stream` | 用户 | HTTP Range 流媒体 |
| GET | `/api/videos/{id}/download` | 用户 | 下载原文件 |
| GET | `/api/videos/{id}/download/remux` | 用户 | 可选轨道的 MP4/MKV |
| GET | `/api/videos/{id}/tracks` | 用户 | 音轨/字幕列表 |
| POST | `/api/uploads` | 管理员 | 创建分片上传会话 |
| PUT | `/api/uploads/{id}/chunk/{index}` | 管理员 | 上传一个分片 |
| POST | `/api/uploads/{id}/complete` | 管理员 | 完成上传 |
| POST | `/api/downloads` | 管理员 | 加入 yt-dlp 下载 |
| GET | `/api/downloads` | 管理员 | 下载任务列表 |
| GET/POST | `/api/admin/blocked-titles` | 管理员 | 标题屏蔽管理 |
| GET | `/api/admin/users` | 管理员 | 用户管理 |
| GET | `/api/healthz` | 公开 | 健康检查 |

完整路由表与数据流见 [architecture.zh-CN.md](architecture.zh-CN.md)。
