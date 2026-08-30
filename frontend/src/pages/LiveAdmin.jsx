import { useCallback, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function LiveAdmin() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [title, setTitle] = useState('');
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  const refresh = useCallback(() => {
    api('/live').then((d) => setItems(d.items || [])).catch(() => {});
  }, []);
  useEffect(() => {
    refresh();
  }, [refresh]);
  const active = items.some((s) => s.status === 'live' || s.status === 'starting');
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(refresh, 4000);
    return () => clearInterval(timer);
  }, [active, refresh]);

  async function add(e) {
    e.preventDefault();
    setErr('');
    setMsg('');
    try {
      const d = await api('/live', { method: 'POST', body: { title } });
      setMsg(`${t('live.ingest')}: ${d.ingest_url}`);
      setTitle('');
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function setRunning(s, running) {
    setErr('');
    try {
      await api(`/live/${s.id}/${running ? 'start' : 'stop'}`, { method: 'POST' });
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card inline-form" onSubmit={add}>
        <input placeholder={t('live.titlePlaceholder')} value={title} onChange={(e) => setTitle(e.target.value)} required />
        <button className="btn primary">{t('live.create')}</button>
      </form>
      {items.length === 0 ? (
        <div className="empty">{t('live.empty')}</div>
      ) : (
        <div className="playlist-list">
          {items.map((s) => (
            <div key={s.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">🔴</div>
                <div>
                  <div className="playlist-name">
                    {s.title}
                    <span className={`status-badge status-${s.status}`}>
                      {t(`live.status${s.status[0].toUpperCase()}${s.status.slice(1)}`)}
                    </span>
                  </div>
                  {s.ingest_url && <div className="muted mono">{t('live.ingest')}: {s.ingest_url}</div>}
                  {s.error && <div className="form-error small">{s.error}</div>}
                </div>
              </div>
              <div className="detail-actions">
                <Link className="btn ghost" to={`/live/${s.id}`}>{t('live.watch')}</Link>
                {s.status === 'live' || s.status === 'starting' ? (
                  <button className="btn ghost" onClick={() => setRunning(s, false)}>{t('live.stop')}</button>
                ) : (
                  <button className="btn ghost" onClick={() => setRunning(s, true)}>{t('live.start')}</button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
