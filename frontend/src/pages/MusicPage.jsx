import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';
import { fmtBytes, fmtDuration } from '../i18n';
import { PlayIcon } from '../components/Icons.jsx';

export default function MusicPage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const [albums, setAlbums] = useState([]);
  const [album, setAlbum] = useState(null);
  const [tracks, setTracks] = useState([]);
  const [err, setErr] = useState('');

  useEffect(() => {
    setErr('');
    if (id) {
      api(`/albums/${id}`)
        .then((d) => {
          setAlbum(d.album);
          setTracks(d.tracks || []);
        })
        .catch((e) => setErr(e.message));
    } else {
      api('/albums')
        .then((d) => setAlbums(d.items || []))
        .catch((e) => setErr(e.message));
    }
  }, [id]);

  if (id) {
    if (err && !album) return <div className="container"><div className="form-error">{err}</div></div>;
    if (!album) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;
    return (
      <div className="container">
        <div className="detail">
          <img
            className="detail-poster album-cover"
            src={album.has_cover ? mediaUrl(`/albums/${album.id}/poster`) : undefined}
            alt=""
          />
          <div className="detail-info">
            <div className="detail-meta-top">
              <span className="library-tag">{album.library_name}</span>
              {album.year > 0 && <span className="year-tag">{album.year}</span>}
            </div>
            <h1>{album.name}</h1>
            <p className="synopsis">{album.artist}</p>
            <div className="detail-actions">
              {tracks.length > 0 && (
                <Link className="btn primary big" to={`/player/${tracks[0].id}?album=${album.id}`}>
                  <PlayIcon />
                  {t('music.playAll')}
                </Link>
              )}
            </div>
          </div>
        </div>
        <div className="card">
          {tracks.length === 0 && <div className="empty">{t('music.noTracks')}</div>}
          {tracks.map((tr) => (
            <div key={tr.id} className="playlist-row">
              <div className="playlist-item-main">
                <b>{tr.title}</b>
                <span className="muted small">
                  {[tr.artist, tr.duration_sec > 0 ? fmtDuration(tr.duration_sec) : '', tr.size_bytes > 0 ? fmtBytes(tr.size_bytes) : '']
                    .filter(Boolean)
                    .join(' · ')}
                </span>
              </div>
              <Link className="btn small primary" to={`/player/${tr.id}?album=${album.id}`}>
                <PlayIcon />
                {t('music.play')}
              </Link>
            </div>
          ))}
        </div>
        <div className="back-link">
          <Link to="/music">← {t('nav.music')}</Link>
        </div>
      </div>
    );
  }

  return (
    <div className="container">
      <h1>{t('nav.music')}</h1>
      {err && <div className="form-error">{err}</div>}
      {albums.length === 0 && !err && <div className="empty">{t('music.noAlbums')}</div>}
      <div className="video-grid">
        {albums.map((al) => (
          <Link key={al.id} to={`/music/${al.id}`} className="poster-link">
            <div className="album-card">
              <img
                className="album-cover"
                src={al.has_cover ? mediaUrl(`/albums/${al.id}/poster`) : undefined}
                alt=""
              />
              <div className="album-name">{al.name}</div>
              <div className="muted small">
                {[al.artist, al.track_count > 0 ? `${al.track_count} ${t('music.tracks')}` : '']
                  .filter(Boolean)
                  .join(' · ')}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
