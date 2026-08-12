import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import Poster from '../components/Poster.jsx';
import ShareModal from '../components/ShareModal.jsx';
import { fmtDuration } from '../i18n';

export default function PlaylistDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [playlist, setPlaylist] = useState(null);
  const [items, setItems] = useState([]);
  const [err, setErr] = useState('');
  const [showShare, setShowShare] = useState(false);

  useEffect(() => {
    api(`/playlists/${id}`)
      .then((d) => {
        setPlaylist(d.playlist);
        setItems(d.items);
      })
      .catch((e) => setErr(e.message));
  }, [id]);

  async function removeItem(videoId) {
    try {
      await api(`/playlists/${id}/items/${videoId}`, { method: 'DELETE' });
      setItems((prev) => prev.filter((i) => i.video.id !== videoId));
      setPlaylist((p) => ({ ...p, item_count: p.item_count - 1 }));
    } catch (e) {
      setErr(e.message);
    }
  }

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!playlist) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  return (
    <div className="container">
      <div className="playlist-head">
        <Link to="/playlists" className="btn ghost">{t('playlists.back')}</Link>
        <div>
          <h1>{playlist.name}</h1>
          {playlist.description && <p className="muted">{playlist.description}</p>}
        </div>
        <div className="detail-actions">
          {items.length > 0 && (
            <button
              className="btn primary"
              onClick={() => navigate(`/player/${items[0].video.id}?playlist=${id}`)}
            >
              {t('playlists.playAll')}
            </button>
          )}
          <button className="btn" onClick={() => setShowShare(true)}>
            {t('video.share')}
          </button>
        </div>
      </div>
      {showShare && <ShareModal kind="playlists" id={playlist.id} onClose={() => setShowShare(false)} />}

      {items.length === 0 ? (
        <div className="empty">{t('playlists.detailEmpty')}</div>
      ) : (
        <div className="playlist-items">
          {items.map((item, idx) => (
            <div key={item.video.id} className="card playlist-item">
              <span className="queue-idx">{idx + 1}</span>
              <Link to={`/video/${item.video.id}`} className="playlist-item-main">
                <Poster video={item.video} className="thumb" />
                <div>
                  <div className="playlist-name">{item.video.title}</div>
                  <div className="muted">
                    {item.video.year > 0 && `${item.video.year} · `}
                    {item.video.duration_sec > 0 && `${fmtDuration(item.video.duration_sec)} · `}
                    {item.video.library_name}
                  </div>
                </div>
              </Link>
              <div className="playlist-item-actions">
                <button
                  className="btn small"
                  onClick={() => navigate(`/player/${item.video.id}?playlist=${id}`)}
                >
                  {t('playlists.play')}
                </button>
                <button className="btn small danger-ghost" onClick={() => removeItem(item.video.id)}>
                  {t('playlists.remove')}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
