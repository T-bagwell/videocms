import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

export default function PhotosPage() {
  const { albumId } = useParams();
  const { t } = useTranslation();
  const [albums, setAlbums] = useState([]);
  const [photos, setPhotos] = useState([]);
  const [err, setErr] = useState('');
  const [slide, setSlide] = useState(-1);
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    setErr('');
    if (albumId) {
      api(`/photos?album_id=${albumId}`)
        .then((d) => setPhotos(d.items || []))
        .catch((e) => setErr(e.message));
    } else {
      api('/photo-albums')
        .then((d) => setAlbums(d.items || []))
        .catch((e) => setErr(e.message));
    }
    setSlide(-1);
    setPlaying(false);
  }, [albumId]);

  useEffect(() => {
    if (slide < 0 || !playing) return;
    const iv = setInterval(() => {
      setSlide((s) => (s < 0 ? -1 : (s + 1) % photos.length));
    }, 3000);
    return () => clearInterval(iv);
  }, [slide, playing, photos.length]);

  useEffect(() => {
    function onKey(e) {
      if (slide < 0) return;
      const tag = (e.target.tagName || '').toLowerCase();
      if (tag === 'input' || tag === 'textarea') return;
      if (e.key === 'ArrowRight' || e.key === ' ') {
        e.preventDefault();
        setSlide((s) => (s + 1) % photos.length);
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        setSlide((s) => (s - 1 + photos.length) % photos.length);
      } else if (e.key === 'Escape') {
        setSlide(-1);
        setPlaying(false);
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [slide, photos.length]);

  if (albumId) {
    return (
      <div className="container">
        <div className="reader-head">
          <h1>{t('nav.photos')}</h1>
          <div>
            <Link className="btn small ghost" to="/photos">← {t('nav.photos')}</Link>
            {photos.length > 0 && (
              <button
                className="btn small primary"
                onClick={() => {
                  setSlide(0);
                  setPlaying(true);
                }}
              >
                {t('photos.slideshow')}
              </button>
            )}
          </div>
        </div>
        {err && <div className="form-error">{err}</div>}
        {photos.length === 0 && !err && <div className="empty">{t('photos.noPhotos')}</div>}
        <div className="video-grid">
          {photos.map((p, i) => (
            <button
              key={p.id}
              className="photo-cell"
              onClick={() => {
                setSlide(i);
                setPlaying(false);
              }}
            >
              <img src={mediaUrl(`/photos/${p.id}/file`)} alt={p.title} loading="lazy" />
            </button>
          ))}
        </div>
        {slide >= 0 && photos[slide] && (
          <div className="slideshow-backdrop" onClick={() => { setSlide(-1); setPlaying(false); }}>
            <div className="slideshow" onClick={(e) => e.stopPropagation()}>
              <img
                className="slideshow-img"
                src={mediaUrl(`/photos/${photos[slide].id}/file`)}
                alt={photos[slide].title}
              />
              <div className="slideshow-tools">
                <button className="btn small ghost" onClick={() => setSlide((slide - 1 + photos.length) % photos.length)}>
                  {t('photos.prev')}
                </button>
                <span className="muted small">
                  {t('photos.of', { current: slide + 1, total: photos.length })}
                </span>
                <button className="btn small ghost" onClick={() => setSlide((slide + 1) % photos.length)}>
                  {t('photos.next')}
                </button>
                <button className="btn small ghost" onClick={() => setPlaying((v) => !v)}>
                  {playing ? t('photos.pause') : t('photos.play')}
                </button>
                <button className="btn small ghost" onClick={() => { setSlide(-1); setPlaying(false); }}>
                  {t('common.close')}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="container">
      <h1>{t('nav.photos')}</h1>
      {err && <div className="form-error">{err}</div>}
      {albums.length === 0 && !err && <div className="empty">{t('photos.noAlbums')}</div>}
      <div className="video-grid">
        {albums.map((al) => (
          <Link key={al.id} to={`/photos/${al.id}`} className="poster-link">
            <div className="album-card">
              <img
                className="album-cover"
                src={al.has_cover ? mediaUrl(`/photos/${al.cover_photo_id}/file`) : undefined}
                alt=""
                loading="lazy"
              />
              <div className="album-name">{al.name}</div>
              <div className="muted small">{al.photo_count} {t('photos.photos')}</div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
