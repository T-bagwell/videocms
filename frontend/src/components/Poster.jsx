import { mediaUrl } from '../api.js';

export default function Poster({ video, className = '' }) {
  if (!video?.has_poster) {
    return (
      <div className={`poster placeholder ${className}`}>
        <span>🎬</span>
      </div>
    );
  }
  return (
    <img
      className={`poster ${className}`}
      src={mediaUrl(`/videos/${video.id}/poster`)}
      alt={video.title}
      loading="lazy"
    />
  );
}

