package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"videocms/backend/internal/auth"
	"videocms/backend/internal/models"
)

func (a *App) listSeries(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	args := []any{user.ID}
	libFilter := ""
	if libID := r.URL.Query().Get("library_id"); libID != "" {
		if id, err := uuid.Parse(libID); err == nil {
			libFilter = " AND s.library_id = $2"
			args = append(args, id)
		}
	}
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT s.id, s.library_id, l.name, s.name, s.season,
		       (SELECT count(*) FROM videos v WHERE v.series_id=s.id AND %s) AS episode_count,
		       s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND %s
		        ORDER BY v.season, v.episode LIMIT 1) AS poster_path,
		       EXISTS(SELECT 1 FROM series_favorites sf
		              WHERE sf.user_id=$1 AND sf.series_id=s.id) AS is_fav,
		       EXISTS(SELECT 1 FROM channel_subscriptions cs
		              WHERE cs.user_id=$1 AND cs.series_id=s.id) AS is_subscribed,
		       (SELECT count(*) FROM channel_subscriptions cs2 WHERE cs2.series_id=s.id) AS subscribers
		FROM series s
		JOIN libraries l ON l.id = s.library_id
		WHERE EXISTS (SELECT 1 FROM videos v WHERE v.series_id=s.id AND %s)%s
		ORDER BY COALESCE(
			(SELECT max(v.created_at) FROM videos v
			 WHERE v.series_id = s.id AND %s),
			s.created_at) DESC, lower(s.name), s.season`,
		visibleEpisodes(1), visibleEpisodes(1), visibleEpisodes(1), libFilter, visibleEpisodes(1)), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query series failed")
		return
	}
	defer rows.Close()

	items := []models.Series{}
	for rows.Next() {
		var s models.Series
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
			&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath, &s.IsFavorite, &s.IsSubscribed, &s.Subscribers); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan series failed")
			return
		}
		s.HasPoster = s.PosterPath != ""
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) getSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)

	var s models.Series
	err = a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT s.id, s.library_id, l.name, s.name, s.season,
		       (SELECT count(*) FROM videos v WHERE v.series_id=s.id AND %s) AS episode_count,
		       s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND %s
		        ORDER BY v.season, v.episode LIMIT 1),
		       EXISTS(SELECT 1 FROM series_favorites sf
		              WHERE sf.user_id=$2 AND sf.series_id=s.id),
		       EXISTS(SELECT 1 FROM channel_subscriptions cs
		              WHERE cs.user_id=$2 AND cs.series_id=s.id),
		       (SELECT count(*) FROM channel_subscriptions cs2 WHERE cs2.series_id=s.id)
		FROM series s JOIN libraries l ON l.id = s.library_id
		WHERE s.id=$1`, visibleEpisodes(2), visibleEpisodes(2)), id, user.ID).Scan(
		&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
		&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath, &s.IsFavorite, &s.IsSubscribed, &s.Subscribers)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, http.StatusNotFound, "series not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query series failed")
		return
	}
	s.HasPoster = s.PosterPath != ""

	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT %s FROM videos v
		JOIN libraries l ON l.id = v.library_id
		LEFT JOIN series s ON s.id = v.series_id
		%s
		WHERE v.series_id=$2 AND %s
		ORDER BY v.season, v.episode, lower(v.title)`,
		videoColumns, blockedLateral, visibleEpisodes(1)), user.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query episodes failed")
		return
	}
	defer rows.Close()

	items := []models.Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "scan episode failed")
			return
		}
		items = append(items, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"series": s, "items": items})
}

func (a *App) addSeriesFavorite(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`INSERT INTO series_favorites (user_id, series_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, user.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "add series favorite failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "favorited"})
}

func (a *App) removeSeriesFavorite(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM series_favorites WHERE user_id=$1 AND series_id=$2`,
		user.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "remove series favorite failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}

func (a *App) mySeriesFavorites(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT s.id, s.library_id, l.name, s.name, s.season,
		       (SELECT count(*) FROM videos v WHERE v.series_id=s.id AND %s) AS episode_count,
		       s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND %s
		        ORDER BY v.season, v.episode LIMIT 1),
		       true AS is_fav
		FROM series_favorites sf
		JOIN series s ON s.id = sf.series_id
		JOIN libraries l ON l.id = s.library_id
		WHERE sf.user_id=$1
		  AND EXISTS (SELECT 1 FROM videos v WHERE v.series_id=s.id AND %s)
		ORDER BY sf.created_at DESC`,
		visibleEpisodes(1), visibleEpisodes(1), visibleEpisodes(1)), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query series favorites failed")
		return
	}
	defer rows.Close()

	items := []models.Series{}
	for rows.Next() {
		var s models.Series
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
			&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath, &s.IsFavorite); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan series favorite failed")
			return
		}
		s.HasPoster = s.PosterPath != ""
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) subscribeSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`INSERT INTO channel_subscriptions (user_id, series_id) VALUES ($1,$2)
		 ON CONFLICT DO NOTHING`, user.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "subscribe failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "subscribed"})
}

func (a *App) unsubscribeSeries(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	user := auth.UserFrom(r)
	if _, err := a.pool.Exec(r.Context(),
		`DELETE FROM channel_subscriptions WHERE user_id=$1 AND series_id=$2`,
		user.ID, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "unsubscribe failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "removed"})
}

func (a *App) mySeriesSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r)
	rows, err := a.pool.Query(r.Context(), fmt.Sprintf(`
		SELECT s.id, s.library_id, l.name, s.name, s.season,
		       (SELECT count(*) FROM videos v WHERE v.series_id=s.id AND %s) AS episode_count,
		       s.updated_at,
		       (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND %s
		        ORDER BY v.season, v.episode LIMIT 1),
		       true AS is_fav,
		       true AS is_subscribed,
		       (SELECT count(*) FROM channel_subscriptions cs2 WHERE cs2.series_id=s.id) AS subscribers
		FROM channel_subscriptions cs
		JOIN series s ON s.id = cs.series_id
		JOIN libraries l ON l.id = s.library_id
		WHERE cs.user_id=$1
		  AND EXISTS (SELECT 1 FROM videos v WHERE v.series_id=s.id AND %s)
		ORDER BY cs.created_at DESC`,
		visibleEpisodes(1), visibleEpisodes(1), visibleEpisodes(1)), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "query subscriptions failed")
		return
	}
	defer rows.Close()
	items := []models.Series{}
	for rows.Next() {
		var s models.Series
		if err := rows.Scan(&s.ID, &s.LibraryID, &s.LibraryName, &s.Name, &s.Season,
			&s.EpisodeCount, &s.UpdatedAt, &s.PosterPath, &s.IsFavorite, &s.IsSubscribed, &s.Subscribers); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan series failed")
			return
		}
		s.HasPoster = s.PosterPath != ""
		items = append(items, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *App) seriesPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid series id")
		return
	}
	var path string
	err = a.pool.QueryRow(r.Context(), fmt.Sprintf(`
		SELECT (SELECT v.poster_path FROM videos v
		        WHERE v.series_id = s.id AND %s
		        ORDER BY v.season, v.episode LIMIT 1)
		FROM series s WHERE s.id=$1`, visibleEpisodes(2)), id, auth.UserFrom(r).ID).Scan(&path)
	if err != nil || path == "" {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, path)
}
