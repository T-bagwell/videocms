import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, apiBaseUrl, publicUrl } from '../api.js';
import { fmtDuration } from '../i18n';

export default function PublicVideoPage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const [video, setVideo] = useState(null);
  const [token, setToken] = useState('');
  const [password, setPassword] = useState('');
  const [needPassword, setNeedPassword] = useState(false);
  const [err, setErr] = useState('');

  const load = useCallback((vt) => {
    const headers = vt ? { Authorization: `Bearer ${vt}` } : {};
    fetch(publicUrl(`/videos/${id}`), { headers })
      .then(async (res) => {
        if (res.status === 401) {
          setNeedPassword(true);
          throw new Error(t('public.passwordTitle'));
        }
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((d) => {
        setVideo(d);
        setErr('');
        setNeedPassword(false);
      })
      .catch((e) => setErr(e.message));
  }, [id, t]);

  useEffect(() => {
    setVideo(null);
    setToken('');
    setNeedPassword(false);
    setErr('');
    load('');
  }, [id, load]);

  async function unlock(e) {
    e.preventDefault();
    setErr('');
    try {
      const d = await api(`/public/videos/${id}/unlock`, {
        method: 'POST',
        body: { password },
      });
      setToken(d.token);
      load(d.token);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  if (err && !video && !needPassword) {
    return (
      <div className="container">
        <div className="form-error">{err}</div>
        <div className="back-link"><Link to="/public">← {t('public.title')}</Link></div>
      </div>
    );
  }
  if (!video && !needPassword) {
    return <div className="container"><div className="loading">{t('common.loading')}</div></div>;
  }
  if (needPassword) {
    return (
      <div className="container">
        <h1>{t('public.passwordTitle')}</h1>
        {err && <div className="form-error">{err}</div>}
        <form className="inline-form" onSubmit={unlock}>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t('public.passwordPlaceholder')}
          />
          <button className="btn primary">{t('public.unlock')}</button>
        </form>
        <div className="back-link"><Link to="/public">← {t('public.title')}</Link></div>
      </div>
    );
  }

  const streamUrl = `${apiBaseUrl()}/api/public/videos/${id}/stream${token ? `?vt=${encodeURIComponent(token)}` : ''}`;
  return (
    <div className="container">
      <div className="detail">
        {video.has_poster && (
          <img
            className="detail-poster"
            src={`${apiBaseUrl()}/api/public/videos/${id}/poster${token ? `?vt=${encodeURIComponent(token)}` : ''}`}
            alt=""
          />
        )}
        <div className="detail-info">
          <h1>{video.title}</h1>
          <div className="detail-facts">
            {video.year > 0 && <span>{video.year}</span>}
            {video.duration_sec > 0 && <span>{fmtDuration(video.duration_sec)}</span>}
            {video.width > 0 && <span>{video.width}×{video.height}</span>}
            {video.video_codec && <span>{video.video_codec}</span>}
            {video.genres?.length > 0 && <span>{video.genres.join(' / ')}</span>}
          </div>
          <div className="player-wrap">
            <video className="player" controls autoPlay src={streamUrl} />
          </div>
        </div>
      </div>
      <div className="back-link"><Link to="/public">← {t('public.title')}</Link></div>
    </div>
  );
}
