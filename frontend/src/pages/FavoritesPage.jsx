import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import VideoCard from '../components/VideoCard.jsx';
import SeriesCard from '../components/SeriesCard.jsx';

export default function FavoritesPage() {
  const { t } = useTranslation();
  const [videos, setVideos] = useState([]);
  const [series, setSeries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');

  useEffect(() => {
    Promise.all([api('/users/me/favorites'), api('/users/me/series-favorites')])
      .then(([v, s]) => {
        setVideos(v.items);
        setSeries(s.items);
      })
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="container">
      <h1>{t('favorites.title')}</h1>
      {err && <div className="form-error">{err}</div>}
      {loading ? (
        <div className="loading">{t('common.loading')}</div>
      ) : (
        <>
          {videos.length > 0 && (
            <section className="section">
              <h2>{t('favorites.videosTitle')}</h2>
              <div className="card-grid">
                {videos.map((v) => (
                  <VideoCard key={v.id} video={v} />
                ))}
              </div>
            </section>
          )}
          {series.length > 0 && (
            <section className="section">
              <h2>{t('favorites.seriesTitle')}</h2>
              <div className="card-grid">
                {series.map((s) => (
                  <SeriesCard key={s.id} series={s} />
                ))}
              </div>
            </section>
          )}
          {videos.length === 0 && series.length === 0 && (
            <div className="empty">{t('favorites.empty')}</div>
          )}
        </>
      )}
    </div>
  );
}
