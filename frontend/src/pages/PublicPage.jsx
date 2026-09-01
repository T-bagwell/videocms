import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, apiBaseUrl } from '../api.js';
import { fmtDuration } from '../i18n';

export default function PublicPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/public/videos')
      .then((d) => setItems(d.items || []))
      .catch((e) => setErr(e.message));
  }, []);

  return (
    <div className="container">
      <h1>{t('public.title')}</h1>
      {err && <div className="form-error">{err}</div>}
      {items.length === 0 && !err && <div className="empty">{t('public.empty')}</div>}
      <div className="video-grid">
        {items.map((v) => (
          <Link key={v.id} to={`/public/v/${v.id}`} className="poster-link">
            <img
              src={v.has_poster ? `${apiBaseUrl()}/api/public/videos/${v.id}/poster` : undefined}
              alt={v.title}
            />
            <div className="album-name">{v.title}</div>
            <div className="muted small">
              {[v.year > 0 ? v.year : '', v.duration_sec > 0 ? fmtDuration(v.duration_sec) : '']
                .filter(Boolean)
                .join(' · ')}
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
