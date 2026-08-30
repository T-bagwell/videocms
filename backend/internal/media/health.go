package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DuplicateGroup groups files that look like duplicates (same declared size).
type DuplicateGroup struct {
	Size  int64    `json:"size"`
	Count int      `json:"count"`
	Files []string `json:"files"`
}

// HealthReport is the result of a library health check.
type HealthReport struct {
	Checked    int              `json:"checked"`
	Missing    []string         `json:"missing"`
	Corrupt    []string         `json:"corrupt"`
	Duplicates []DuplicateGroup `json:"duplicates"`
}

type healthVideo struct {
	ID      uuid.UUID
	Path    string
	Size    int64
	Width   int
	Height  int
	Missing bool
	Corrupt bool
}

// RunHealthCheck inspects a library's available videos for missing/corrupt
// files and duplicate candidates (same declared size).
func RunHealthCheck(ctx context.Context, pool *pgxpool.Pool, libraryID uuid.UUID) (HealthReport, error) {
	videos, err := loadHealthVideos(ctx, pool, libraryID)
	if err != nil {
		return HealthReport{}, err
	}
	report := HealthReport{Checked: len(videos)}
	bySize := map[int64][]healthVideo{}
	for _, v := range videos {
		if _, err := os.Stat(v.Path); err != nil {
			v.Missing = true
			report.Missing = append(report.Missing, v.Path)
			continue
		}
		if v.Size <= 0 {
			v.Corrupt = true
			report.Corrupt = append(report.Corrupt, v.Path)
		}
		bySize[v.Size] = append(bySize[v.Size], v)
	}
	sizes := make([]int64, 0, len(bySize))
	for size := range bySize {
		sizes = append(sizes, size)
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i] < sizes[j] })
	for _, size := range sizes {
		group := bySize[size]
		if len(group) < 2 {
			continue
		}
		files := make([]string, 0, len(group))
		for _, v := range group {
			files = append(files, v.Path)
		}
		sort.Strings(files)
		report.Duplicates = append(report.Duplicates, DuplicateGroup{Size: size, Count: len(files), Files: files})
	}
	return report, nil
}

// KeepBestMoves moves duplicate candidates except the best one per group into
// DATA_DIR/trash/<date>/ and marks the moved videos unavailable.
func KeepBestMoves(ctx context.Context, pool *pgxpool.Pool, dataDir string, libraryID uuid.UUID) ([]string, []string, error) {
	videos, err := loadHealthVideos(ctx, pool, libraryID)
	if err != nil {
		return nil, nil, err
	}
	bySize := map[int64][]healthVideo{}
	for _, v := range videos {
		if _, err := os.Stat(v.Path); err != nil {
			continue
		}
		bySize[v.Size] = append(bySize[v.Size], v)
	}
	trashDir := filepath.Join(dataDir, "trash", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return nil, nil, err
	}
	moved := []string{}
	errorsList := []string{}
	for _, group := range bySize {
		if len(group) < 2 {
			continue
		}
		// Best = highest resolution, then longest filename.
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Width*group[i].Height != group[j].Width*group[j].Height {
				return group[i].Width*group[i].Height > group[j].Width*group[j].Height
			}
			return len(filepath.Base(group[i].Path)) > len(filepath.Base(group[j].Path))
		})
		for _, v := range group[1:] {
			dst := filepath.Join(trashDir, filepath.Base(v.Path))
			if _, err := os.Stat(dst); err == nil {
				dst = filepath.Join(trashDir, fmt.Sprintf("%s-%s", v.ID.String()[:8], filepath.Base(v.Path)))
			}
			if err := os.Rename(v.Path, dst); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("%s: %v", v.Path, err))
				continue
			}
			if _, err := pool.Exec(ctx,
				`UPDATE videos SET available=false WHERE id=$1`, v.ID); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("update %s: %v", v.Path, err))
			}
			moved = append(moved, dst)
		}
	}
	return moved, errorsList, nil
}

func loadHealthVideos(ctx context.Context, pool *pgxpool.Pool, libraryID uuid.UUID) ([]healthVideo, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, file_path, size_bytes, width, height
		FROM videos WHERE library_id=$1 AND available`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	videos := []healthVideo{}
	for rows.Next() {
		var v healthVideo
		if err := rows.Scan(&v.ID, &v.Path, &v.Size, &v.Width, &v.Height); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}
