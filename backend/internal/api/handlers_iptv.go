package api

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
)

type iptvChannel struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	TvgID      string     `json:"tvg_id"`
	TvgName    string     `json:"tvg_name"`
	Logo       string     `json:"logo"`
	GroupTitle string     `json:"group_title"`
	SourceURL  string     `json:"source_url"`
	LibraryID  *uuid.UUID `json:"library_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func scanIptvChannel(row pgx.Row) (iptvChannel, error) {
	var c iptvChannel
	err := row.Scan(&c.ID, &c.Name, &c.TvgID, &c.TvgName, &c.Logo, &c.GroupTitle,
		&c.SourceURL, &c.LibraryID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// GET /api/iptv/channels
func (a *App) listIptvChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, tvg_id, tvg_name, logo, group_title, source_url, library_id, created_at, updated_at
		FROM iptv_channels ORDER BY lower(name)`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query iptv channels failed")
		return
	}
	defer rows.Close()
	items := []iptvChannel{}
	for rows.Next() {
		c, err := scanIptvChannel(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan iptv channel failed")
			return
		}
		items = append(items, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// POST /api/iptv/channels
func (a *App) createIptvChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		TvgID      string `json:"tvg_id"`
		TvgName    string `json:"tvg_name"`
		Logo       string `json:"logo"`
		GroupTitle string `json:"group_title"`
		SourceURL  string `json:"source_url"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || strings.TrimSpace(req.SourceURL) == "" {
		writeErr(w, http.StatusBadRequest, "name and source_url are required")
		return
	}
	var c iptvChannel
	err := a.pool.QueryRow(r.Context(), `
		INSERT INTO iptv_channels (name, tvg_id, tvg_name, logo, group_title, source_url)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, name, tvg_id, tvg_name, logo, group_title, source_url, library_id, created_at, updated_at`,
		req.Name, strings.TrimSpace(req.TvgID), strings.TrimSpace(req.TvgName), strings.TrimSpace(req.Logo),
		strings.TrimSpace(req.GroupTitle), strings.TrimSpace(req.SourceURL)).Scan(&c.ID, &c.Name, &c.TvgID,
		&c.TvgName, &c.Logo, &c.GroupTitle, &c.SourceURL, &c.LibraryID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create iptv channel failed")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// DELETE /api/iptv/channels/{id}
func (a *App) deleteIptvChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM iptv_channels WHERE id=$1`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete iptv channel failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "deleted"})
}

// POST /api/iptv/import parses an M3U playlist and upserts its channels.
func (a *App) importIptvM3U(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := a.fetchText(r.Context(), strings.TrimSpace(req.URL), 4<<20)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "fetch m3u failed")
		return
	}
	channels := parseM3U(string(data))
	if len(channels) == 0 {
		writeErr(w, http.StatusBadRequest, "no channels found in playlist")
		return
	}
	count := 0
	for _, c := range channels {
		if _, err := a.pool.Exec(r.Context(), `
			INSERT INTO iptv_channels (name, tvg_id, tvg_name, logo, group_title, source_url)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT (source_url) WHERE source_url <> '' DO UPDATE SET
				name=EXCLUDED.name, tvg_id=EXCLUDED.tvg_id, tvg_name=EXCLUDED.tvg_name,
				logo=EXCLUDED.logo, group_title=EXCLUDED.group_title, updated_at=now()`,
			c.name, c.tvgID, c.tvgName, c.logo, c.group, c.source); err != nil {
			log.Printf("import m3u channel %q: %v", c.name, err)
		} else {
			count++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": count, "found": len(channels)})
}

var extinfAttrRe = regexp.MustCompile(`(\w[\w-]*)\s*=\s*"([^"]*)"`)

type m3uChannel struct {
	name, tvgID, tvgName, logo, group, source string
}

func parseM3U(data string) []m3uChannel {
	var out []m3uChannel
	var cur m3uChannel
	haveName := false
	for _, raw := range strings.Split(data, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#EXTINF") {
			for _, m := range extinfAttrRe.FindAllStringSubmatch(line, -1) {
				switch m[1] {
				case "tvg-id":
					cur.tvgID = m[2]
				case "tvg-name":
					cur.tvgName = m[2]
				case "tvg-logo":
					cur.logo = m[2]
				case "group-title":
					cur.group = m[2]
				}
			}
			if i := strings.LastIndex(line, ","); i >= 0 {
				cur.name = strings.TrimSpace(line[i+1:])
				haveName = true
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if cur.source == "" {
			cur.source = line
			if !haveName {
				cur.name = line
			}
			out = append(out, cur)
			cur = m3uChannel{}
			haveName = false
		}
	}
	return out
}

// POST /api/iptv/library-channel creates a channel that streams every
// available video in a library, in order.
func (a *App) createLibraryChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LibraryID  string `json:"library_id"`
		Name       string `json:"name"`
		TvgID      string `json:"tvg_id"`
		GroupTitle string `json:"group_title"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid library_id")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	var c iptvChannel
	err = a.pool.QueryRow(r.Context(), `
		INSERT INTO iptv_channels (name, tvg_id, tvg_name, group_title, library_id)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, name, tvg_id, tvg_name, logo, group_title, source_url, library_id, created_at, updated_at`,
		req.Name, strings.TrimSpace(req.TvgID), strings.TrimSpace(req.TvgID), strings.TrimSpace(req.GroupTitle), libID).
		Scan(&c.ID, &c.Name, &c.TvgID, &c.TvgName, &c.Logo, &c.GroupTitle, &c.SourceURL,
			&c.LibraryID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create library channel failed")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// GET /api/iptv/channels.m3u
func (a *App) iptvChannelsM3U(w http.ResponseWriter, r *http.Request) {
	if !a.iptvAuthed(r) {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT id, name, tvg_id, tvg_name, logo, group_title, source_url, library_id
		FROM iptv_channels ORDER BY lower(name)`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query channels failed")
		return
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	token := r.URL.Query().Get("token")
	base := "http://" + r.Host
	if r.TLS != nil {
		base = "https://" + r.Host
	}
	for rows.Next() {
		var c iptvChannel
		if err := rows.Scan(&c.ID, &c.Name, &c.TvgID, &c.TvgName, &c.Logo, &c.GroupTitle,
			&c.SourceURL, &c.LibraryID); err != nil {
			continue
		}
		name := c.TvgName
		if name == "" {
			name = c.Name
		}
		fmt.Fprintf(&b, "#EXTINF:-1 tvg-id=\"%s\" tvg-name=\"%s\" tvg-logo=\"%s\" group-title=\"%s\",%s\n",
			xmlEscape(c.TvgID), xmlEscape(name), xmlEscape(c.Logo), xmlEscape(c.GroupTitle), xmlEscape(c.Name))
		if c.LibraryID != nil {
			fmt.Fprintf(&b, "%s/api/iptv/library/%s/stream?token=%s\n", base, c.ID, token)
		} else {
			b.WriteString(c.SourceURL)
			b.WriteString("\n")
		}
	}
	w.Header().Set("Content-Type", "audio/x-mpegurl")
	_, _ = w.Write([]byte(b.String()))
}

// GET /api/iptv/library/{id}/stream streams a library's videos in order as a
// continuous MPEG-TS feed.
func (a *App) libraryChannelStream(w http.ResponseWriter, r *http.Request) {
	if !a.iptvAuthed(r) {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	var libID uuid.UUID
	err = a.pool.QueryRow(r.Context(),
		`SELECT library_id FROM iptv_channels WHERE id=$1 AND library_id IS NOT NULL`, id).Scan(&libID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "channel not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query channel failed")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT file_path FROM videos WHERE library_id=$1 AND available=true
		ORDER BY lower(title), created_at`, libID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query library videos failed")
		return
	}
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			paths = append(paths, p)
		}
	}
	rows.Close()
	if len(paths) == 0 {
		writeErr(w, http.StatusNotFound, "library has no videos")
		return
	}

	dir, err := os.MkdirTemp("", "iptv-concat-*")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create temp dir failed")
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	listFile := filepath.Join(dir, "list.txt")
	var lb strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&lb, "file '%s'\n", strings.ReplaceAll(p, "'", "'\\''"))
	}
	if err := os.WriteFile(listFile, []byte(lb.String()), 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "write concat list failed")
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	cmd := exec.CommandContext(ctx, resolveFFmpeg(), "-re", "-v", "error",
		"-f", "concat", "-safe", "0", "-i", listFile,
		"-c", "copy", "-f", "mpegts", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "start stream failed")
		return
	}
	if err := cmd.Start(); err != nil {
		writeErr(w, http.StatusInternalServerError, "start stream failed")
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, stdout)
	_ = cmd.Wait()
}

func resolveFFmpeg() string {
	if b := os.Getenv("FFMPEG_BIN"); b != "" {
		return b
	}
	if p := filepath.Join("/usr/local/opt/ffmpeg/bin", "ffmpeg"); fileExists(p) {
		return p
	}
	return "ffmpeg"
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// POST /api/iptv/epg/import parses an XMLTV file and replaces the programme
// index for the channels it contains.
func (a *App) importIptvEPG(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	data, err := a.fetchText(r.Context(), strings.TrimSpace(req.URL), 16<<20)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "fetch xmltv failed")
		return
	}
	programmes, err := parseXMLTV(string(data))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "parse xmltv failed")
		return
	}
	for _, p := range programmes {
		if _, err := a.pool.Exec(r.Context(), `
			INSERT INTO iptv_programmes (channel_id, start_utc, end_utc, title, description)
			VALUES ($1,$2,$3,$4,$5)`,
			p.channel, p.start, p.end, p.title, p.desc); err != nil {
			log.Printf("import epg programme: %v", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(programmes)})
}

type epgProgramme struct {
	channel string
	start   time.Time
	end     time.Time
	title   string
	desc    string
}

type xmltvRoot struct {
	Channels []struct {
		ID           string   `xml:"id,attr"`
		DisplayNames []string `xml:"display-name"`
	} `xml:"channel"`
	Programmes []struct {
		Channel string   `xml:"channel,attr"`
		Start   string   `xml:"start,attr"`
		End     string   `xml:"end,attr"`
		Titles  []string `xml:"title"`
		Descs   []string `xml:"desc"`
	} `xml:"programme"`
}

func parseXMLTV(data string) ([]epgProgramme, error) {
	var root xmltvRoot
	if err := xml.Unmarshal([]byte(data), &root); err != nil {
		return nil, err
	}
	var out []epgProgramme
	for _, p := range root.Programmes {
		start, err1 := time.Parse("20060102150405 -0700", p.Start)
		end, err2 := time.Parse("20060102150405 -0700", p.End)
		if err1 != nil || err2 != nil || len(p.Titles) == 0 {
			continue
		}
		out = append(out, epgProgramme{
			channel: p.Channel,
			start:   start.UTC(),
			end:     end.UTC(),
			title:   strings.TrimSpace(p.Titles[0]),
			desc:    strings.TrimSpace(firstNonEmpty(p.Descs)),
		})
	}
	return out, nil
}

func firstNonEmpty(ss []string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// GET /api/iptv/epg.xml
func (a *App) iptvEPGXML(w http.ResponseWriter, r *http.Request) {
	if !a.iptvAuthed(r) {
		writeErr(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		SELECT DISTINCT tvg_id FROM iptv_channels WHERE tvg_id <> ''`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query channels failed")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<tv>\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "  <channel id=\"%s\"><display-name>%s</display-name></channel>\n",
			xmlEscape(id), xmlEscape(id))
		progRows, err := a.pool.Query(r.Context(), `
			SELECT start_utc, end_utc, title, description FROM iptv_programmes
			WHERE channel_id=$1 AND end_utc >= now() - interval '24 hours'
			ORDER BY start_utc LIMIT 200`, id)
		if err != nil {
			continue
		}
		for progRows.Next() {
			var start, end time.Time
			var title, desc string
			if err := progRows.Scan(&start, &end, &title, &desc); err != nil {
				continue
			}
			fmt.Fprintf(&b, "  <programme channel=\"%s\" start=\"%s\" end=\"%s\">\n",
				xmlEscape(id), start.Format("20060102150405 -0700"), end.Format("20060102150405 -0700"))
			fmt.Fprintf(&b, "    <title>%s</title>\n", xmlEscape(title))
			if desc != "" {
				fmt.Fprintf(&b, "    <desc>%s</desc>\n", xmlEscape(desc))
			}
			b.WriteString("  </programme>\n")
		}
		progRows.Close()
	}
	b.WriteString("</tv>\n")
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(b.String()))
}

func (a *App) iptvAuthed(r *http.Request) bool {
	tokenStr := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if tokenStr == "" {
		tokenStr = r.URL.Query().Get("token")
	}
	if tokenStr == "" {
		return false
	}
	_, err := auth.Parse(a.cfg.JWTSecret, tokenStr)
	return err == nil
}

func (a *App) fetchText(ctx context.Context, url string, limit int64) ([]byte, error) {
	if url == "" {
		return nil, errors.New("empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}
