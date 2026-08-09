import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { mediaUrl } from '../api.js';

export default function SeriesCard({ series }) {
  const { t } = useTranslation();
  return (
    <Link to={`/series/${series.id}`} className="video-card">
      <div className="video-card-poster">
        {series.has_poster ? (
          <img
            className="poster"
            src={mediaUrl(`/series/${series.id}/poster`)}
            alt={series.name}
            loading="lazy"
          />
        ) : (
          <div className="poster placeholder"><span>📺</span></div>
        )}
      </div>
      <div className="video-card-info">
        <div className="video-card-title" title={series.name}>
          {series.name}
          {series.season > 0 && <span className="season-badge">{t('series.season', { n: series.season })}</span>}
        </div>
        <div className="video-card-meta">
          <span>{t('series.episodes', { count: series.episode_count })}</span>
          {series.is_favorite && <span className="fav-star">★</span>}
        </div>
      </div>
    </Link>
  );
}
