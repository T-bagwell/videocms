# スクレイパー SDK

VideoCMS では、小さな JSON 契約を実装した外部スクレイパーを管理者がインストールできます。
登録したスクレイパーは、組み込みの TMDB/TVMaze/AniList/Wikipedia/OMDb と同様に
詳細ページで動画ごとに選択できます（`POST /api/videos/{id}/scrape?provider=<名前>`）。

## 契約

スクレイパーはタイトル（と任意の年）を受け取り、JSON を返します：

```json
{
  "title": "External Title",
  "year": 2024,
  "synopsis": "説明文",
  "genres": ["SF", "ドラマ"],
  "poster_url": "https://…/poster.jpg",
  "backdrop_url": "https://…/backdrop.jpg",
  "trailer_url": "https://www.youtube.com/watch?v=…",
  "trailer_title": "公式予告編"
}
```

必須なのは `title` のみで、他はすべて省略可能です。`title` が空の場合は「一致なし」を意味します。

## 2 つのモード

- **URL モード**（`kind: "url"`）：サーバーが `Content-Type: application/json` で
  設定した URL に `{"title": "...", "year": N}` を POST します（`%s` は URL エスケープした
  タイトルに置換）。2xx 以外や不正な JSON はスクレイプ失敗になります。
- **コマンドモード**（`kind: "command"`）：タイトルと年を引数にしてコマンドを実行し
  （`<コマンド> "<タイトル>" 2024`）、stdout から契約 JSON を解析します。

## 登録 API

```bash
curl -X POST /api/admin/scrapers \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name": "my-scraper", "kind": "url", "url": "https://api.example.com/scrape/%s"}'
```

`PATCH /api/admin/scrapers/{id}` に `{"enabled": false}` を送ると削除せずに無効化でき、
`DELETE /api/admin/scrapers/{id}` で削除します。管理コンソールの「スクレイパー」タブでも
同じ操作ができます。無効または未知のプロバイダー名はスクレイプを `502` にします。

スクレイパーはサーバー側で実行されます。信頼できるスクレイパーだけをインストールし、
プロセス一覧に表示されるコマンド引数に資格情報を入れないでください。
