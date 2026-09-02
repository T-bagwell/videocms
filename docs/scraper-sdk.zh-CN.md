# 刮削 SDK

VideoCMS 允许管理员安装实现统一 JSON 契约的外部刮削器。注册后即可像内置的
TMDB/TVMaze/AniList/Wikipedia/OMDb 提供方一样，在详情页按视频选择使用
（`POST /api/videos/{id}/scrape?provider=<名称>`）。

## 契约

刮削器接收标题（和可选年份），必须返回 JSON：

```json
{
  "title": "外部标题",
  "year": 2024,
  "synopsis": "简介文本",
  "genres": ["科幻", "剧情"],
  "poster_url": "https://…/poster.jpg",
  "backdrop_url": "https://…/backdrop.jpg",
  "trailer_url": "https://www.youtube.com/watch?v=…",
  "trailer_title": "官方预告片"
}
```

只有 `title` 必填，其余字段均可选；`title` 为空表示"未匹配"。

## 两种模式

- **URL 模式**（`kind: "url"`）：服务器以 `Content-Type: application/json` 向
  配置的地址 POST `{"title": "...", "year": N}`（`%s` 占位符会被替换为 URL 转义的标题）。
  非 2xx 响应或非法 JSON 会使刮削失败。
- **命令模式**（`kind: "command"`）：服务器以标题和年份作为参数执行命令
  （`<命令> "<标题>" 2024`），并从 stdout 解析契约 JSON。

## 注册 API

```bash
curl -X POST /api/admin/scrapers \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name": "my-scraper", "kind": "url", "url": "https://api.example.com/scrape/%s"}'
```

`PATCH /api/admin/scrapers/{id}` 传 `{"enabled": false}` 可停用而不删除；
`DELETE /api/admin/scrapers/{id}` 删除。管理控制台的"刮削器"标签页提供相同操作。
停用或未知的提供方名称会使刮削返回 `502`。

刮削器在服务器端运行：请只安装可信的刮削器，并避免把凭据放进会出现在进程列表中的命令参数。
