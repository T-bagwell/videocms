import { Link } from 'react-router-dom';
import Poster from './Poster.jsx';
import { fmtBytes, fmtDuration } from '../i18n';

export default function VideoCard({ video }) {
  const pct =
    video.progress_sec > 5 && video.progress_duration_sec > 0
      ? Math.min(95, Math.round((video.progress_sec / video.progress_duration_sec) * 100))
      : 0;
  const episodeLabel =
    video.series_name && video.episode
      ? video.season > 0
        ? `S${String(video.season).padStart(2, '0')}E${String(video.episode).padStart(2, '0')}`
        : `E${String(video.episode).padStart(2, '0')}`
      : '';

  return (
    <Link to={`/video/${video.id}`} className="video-card">
      <div className="video-card-poster">
        <Poster video={video} />
        {video.duration_sec > 0 && (
          <span className="duration-badge">{fmtDuration(video.duration_sec)}</span>
        )}
        {pct > 0 && (
          <div className="progress-bar">
            <div className="progress-fill" style={{ width: `${pct}%` }} />
          </div>
        )}
      </div>
      <div className="video-card-info">
        {video.series_name && (
          <div className="video-card-series" title={video.series_name}>
            📺 {video.series_name}
            {episodeLabel && <span className="episode-tag">{episodeLabel}</span>}
          </div>
        )}
        <div className="video-card-title" title={video.title}>
          {video.title}
        </div>
        <div className="video-card-meta">
          {video.year > 0 && <span>{video.year}</span>}
          {video.genres?.length > 0 && <span>{video.genres.slice(0, 2).join(' / ')}</span>}
          <span>{fmtBytes(video.size_bytes)}</span>
          {video.is_favorite && <span className="fav-star">★</span>}
        </div>
      </div>
    </Link>
  );
}
