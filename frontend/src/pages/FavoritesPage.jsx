import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import VideoCard from '../components/VideoCard.jsx';

export default function FavoritesPage() {
  const { t } = useTranslation();
  const [videos, setVideos] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/users/me/favorites')
      .then((d) => setVideos(d.items))
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="container">
      <h1>{t('favorites.title')}</h1>
      {err && <div className="form-error">{err}</div>}
      {loading ? (
        <div className="loading">{t('common.loading')}</div>
      ) : videos.length === 0 ? (
        <div className="empty">{t('favorites.empty')}</div>
      ) : (
        <div className="card-grid">
          {videos.map((v) => (
            <VideoCard key={v.id} video={v} />
          ))}
        </div>
      )}
    </div>
  );
}
