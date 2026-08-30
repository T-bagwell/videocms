package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"display_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Library struct {
	ID             uuid.UUID  `json:"id"`
	Name           string     `json:"name"`
	Path           string     `json:"path"`
	ScanStatus     string     `json:"scan_status"`
	ScanError      string     `json:"scan_error,omitempty"`
	ScanStartedAt  *time.Time `json:"scan_started_at,omitempty"`
	ScanFinishedAt *time.Time `json:"scan_finished_at,omitempty"`
	VideoCount     int64      `json:"video_count"`
	Blocked        bool       `json:"blocked"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Video struct {
	ID           uuid.UUID  `json:"id"`
	LibraryID    uuid.UUID  `json:"library_id"`
	LibraryName  string     `json:"library_name"`
	Title        string     `json:"title"`
	Filename     string     `json:"filename"`
	FilePath     string     `json:"-"`
	SizeBytes    int64      `json:"size_bytes"`
	DurationSec  float64    `json:"duration_sec"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	VideoCodec   string     `json:"video_codec"`
	Container    string     `json:"container"`
	Year         int        `json:"year"`
	Synopsis     string     `json:"synopsis"`
	Genres       []string   `json:"genres"`
	PosterPath   string     `json:"-"`
	SubtitlePath string     `json:"-"`
	TmdbID       int        `json:"tmdb_id,omitempty"`
	ScrapedAt    *time.Time `json:"scraped_at,omitempty"`
	Available    bool       `json:"available"`
	HasPoster    bool       `json:"has_poster"`
	HasSubtitle  bool       `json:"has_subtitle"`
	IsFavorite   bool       `json:"is_favorite"`
	ProgressSec  float64    `json:"progress_sec"`
	ProgressDur  float64    `json:"progress_duration_sec"`
	BlockedID    string     `json:"blocked_id,omitempty"`
	Blocked      bool       `json:"blocked"`
	SeriesID     *uuid.UUID `json:"series_id,omitempty"`
	SeriesName   string     `json:"series_name,omitempty"`
	Season       int        `json:"season,omitempty"`
	Episode      int        `json:"episode,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type SubtitleTrack struct {
	ID       uuid.UUID `json:"id"`
	VideoID  uuid.UUID `json:"-"`
	Position int       `json:"position"`
	Lang     string    `json:"lang"`
	Title    string    `json:"title"`
	Kind     string    `json:"kind"`
	Format   string    `json:"format,omitempty"`
	IsActive bool      `json:"is_active"`
}

type Series struct {
	ID           uuid.UUID `json:"id"`
	LibraryID    uuid.UUID `json:"library_id"`
	LibraryName  string    `json:"library_name"`
	Name         string    `json:"name"`
	Season       int       `json:"season,omitempty"`
	EpisodeCount int       `json:"episode_count"`
	IsFavorite   bool      `json:"is_favorite"`
	HasPoster    bool      `json:"has_poster"`
	PosterPath   string    `json:"-"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Playlist struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ItemCount   int       `json:"item_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlaylistItem struct {
	PlaylistID uuid.UUID `json:"playlist_id"`
	Position   int       `json:"position"`
	AddedAt    time.Time `json:"added_at"`
	Video      Video     `json:"video"`
}

type Progress struct {
	VideoID     uuid.UUID `json:"video_id"`
	PositionSec float64   `json:"position_sec"`
	DurationSec float64   `json:"duration_sec"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Stats struct {
	Users         int64 `json:"users"`
	Libraries     int64 `json:"libraries"`
	Series        int64 `json:"series"`
	Videos        int64 `json:"videos"`
	VideosMissing int64 `json:"videos_missing"`
	Playlists     int64 `json:"playlists"`
	Favorites     int64 `json:"favorites"`
	TotalBytes    int64 `json:"total_bytes"`
}
