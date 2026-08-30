package media

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"videocms/backend/internal/models"
)

// NFOData is the subset of Kodi-style movie NFO metadata we import/export.
type NFOData struct {
	Title    string   `xml:"title"`
	Year     int      `xml:"year"`
	Synopsis string   `xml:"plot"`
	Genres   []string `xml:"genre"`
}

// NFOFileFor returns the NFO path next to a video file (same base name).
func NFOFileFor(videoPath string) string {
	ext := filepath.Ext(videoPath)
	return strings.TrimSuffix(videoPath, ext) + ".nfo"
}

// WriteNFO writes a Kodi-style movie NFO next to the video.
func WriteNFO(video models.Video) (string, error) {
	path := NFOFileFor(video.FilePath)
	genres := ""
	if len(video.Genres) > 0 {
		genres = strings.Join(video.Genres, " / ")
	}
	doc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>%s</title>
  <year>%d</year>
  <plot>%s</plot>
  <genre>%s</genre>
</movie>
`, xmlEscape(video.Title), video.Year, xmlEscape(video.Synopsis), xmlEscape(genres))
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ReadNFO parses a Kodi-style movie NFO file.
func ReadNFO(path string) (NFOData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return NFOData{}, err
	}
	var d NFOData
	if err := xml.Unmarshal(raw, &d); err != nil {
		return NFOData{}, fmt.Errorf("parse NFO %s: %w", path, err)
	}
	return d, nil
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}
