import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';
import ShareModal from '../components/ShareModal.jsx';

function episodeLabel(season, episode) {
  if (!episode) return '';
  return season > 0
    ? `S${String(season).padStart(2, '0')}E${String(episode).padStart(2, '0')}`
    : `E${String(episode).padStart(2, '0')}`;
}

export default function SeriesDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [series, setSeries] = useState(null);
  const [items, setItems] = useState([]);
  const [err, setErr] = useState('');
  const [showShare, setShowShare] = useState(false);

  async function toggleFavorite() {
    try {
      if (series.is_favorite) {
        await api(`/series/${id}/favorite`, { method: 'DELETE' });
      } else {
        await api(`/series/${id}/favorite`, { method: 'POST' });
      }
      setSeries({ ...series, is_favorite: !series.is_favorite });
    } catch (e) {
      setErr(e.message);
    }
  }

  async function toggleSubscribe() {
    try {
      if (series.is_subscribed) {
        await api(`/series/${id}/subscribe`, { method: 'DELETE' });
      } else {
        await api(`/series/${id}/subscribe`, { method: 'PUT' });
      }
      setSeries({ ...series, is_subscribed: !series.is_subscribed });
    } catch (e) {
      setErr(e.message);
    }
  }

  useEffect(() => {
    api(`/series/${id}`)
      .then((d) => {
        setSeries(d.series);
        setItems(d.items);
      })
      .catch((e) => setErr(e.message));
  }, [id]);

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!series) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  return (
    <div className="container">
      <Link to="/series" className="btn ghost">{t('series.back')}</Link>
      <div className="detail series-detail">
        {series.has_poster ? (
          <img className="poster detail-poster" src={mediaUrl(`/series/${series.id}/poster`)} alt={series.name} />
        ) : (
          <div className="poster placeholder detail-poster"><span>📺</span></div>
        )}
        <div className="detail-info">
          <div className="detail-meta-top">
            <span className="library-tag">{series.library_name}</span>
            {series.season > 0 && <span className="year-tag">{t('series.season', { n: series.season })}</span>}
            <span className="year-tag">{t('series.episodes', { count: series.episode_count })}</span>
          </div>
          <h1>{series.name}</h1>
          {items.length > 0 && (
            <div className="detail-actions">
              <button
                className="btn primary big"
                onClick={() => navigate(`/player/${items[0].id}?series=${id}`)}
              >
                {t('series.playAll')}
              </button>
              <button className="btn" onClick={toggleFavorite}>
                {series.is_favorite ? t('series.unfavorite') : t('series.favorite')}
              </button>
              <button className="btn" onClick={toggleSubscribe}>
                {series.is_subscribed ? t('series.unsubscribe') : t('series.subscribe')}
              </button>
              <button className="btn" onClick={() => setShowShare(true)}>
                {t('video.share')}
              </button>
            </div>
          )}
        </div>
      </div>
      {showShare && <ShareModal kind="series" id={series.id} onClose={() => setShowShare(false)} />}

      <h2>{t('series.episodeList')}</h2>
      <div className="playlist-items">
        {items.map((v) => (
          <div key={v.id} className="card playlist-item">
            <span className="queue-idx">{episodeLabel(v.season, v.episode)}</span>
            <Link to={`/video/${v.id}`} className="playlist-item-main">
              {v.has_poster ? (
                <img className="thumb poster" src={mediaUrl(`/videos/${v.id}/poster`)} alt={v.title} />
              ) : (
                <div className="poster placeholder thumb"><span>🎬</span></div>
              )}
              <div>
                <div className="playlist-name">{v.title}</div>
                <div className="muted">
                  {v.duration_sec > 0 && `${Math.round(v.duration_sec / 60)} min · `}
                  {v.library_name}
                </div>
              </div>
            </Link>
            <div className="playlist-item-actions">
              <button className="btn small" onClick={() => navigate(`/player/${v.id}`)}>
                {t('playlists.play')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
