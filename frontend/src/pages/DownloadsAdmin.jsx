import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import PathPicker from '../components/PathPicker.jsx';

export default function DownloadsAdmin() {
  const { t } = useTranslation();
  const [jobs, setJobs] = useState([]);
  const [url, setUrl] = useState('');
  const [format, setFormat] = useState('');
  const [intervalHours, setIntervalHours] = useState('');
  const [target, setTarget] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  const refresh = useCallback(() => {
    api('/downloads').then((d) => setJobs(d.items || [])).catch(() => {});
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const active = jobs.some((j) => j.status === 'queued' || j.status === 'downloading');
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(refresh, 3000);
    return () => clearInterval(timer);
  }, [active, refresh]);

  async function add(e) {
    e.preventDefault();
    setErr('');
    setMsg('');
    try {
      const body = { url: url.trim(), target_path: target };
      if (format.trim()) body.format = format.trim();
      const hours = parseInt(intervalHours, 10);
      if (hours > 0) body.interval_secs = hours * 3600;
      const d = await api('/downloads', { method: 'POST', body });
      setMsg(t('downloads.added', { url: d.url }));
      setUrl('');
      setFormat('');
      setIntervalHours('');
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function cancel(id) {
    setErr('');
    try {
      await api(`/downloads/${id}`, { method: 'DELETE' });
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function retry(id) {
    setErr('');
    try {
      await api(`/downloads/${id}/retry`, { method: 'POST' });
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
        <input
          placeholder={t('downloads.urlPlaceholder')}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
        />
        <div className="path-field">
          <input
            readOnly
            value={target}
            placeholder={t('downloads.noTarget')}
            required
          />
          <button type="button" className="btn" onClick={() => setPickerOpen(true)}>
            {t('admin.browse')}
          </button>
        </div>
        <input
          placeholder={t('downloads.formatPlaceholder')}
          value={format}
          onChange={(e) => setFormat(e.target.value)}
        />
        <input
          type="number"
          min="0"
          placeholder={t('downloads.intervalPlaceholder')}
          value={intervalHours}
          onChange={(e) => setIntervalHours(e.target.value)}
        />
        <button className="btn primary">{t('downloads.add')}</button>
      </form>
      <p className="muted hint">{t('downloads.hint')}</p>

      {pickerOpen && (
        <PathPicker
          initialPath={target}
          onPick={(p) => {
            setTarget(p);
            setPickerOpen(false);
          }}
          onClose={() => setPickerOpen(false)}
        />
      )}

      {jobs.length === 0 ? (
        <div className="empty">{t('downloads.empty')}</div>
      ) : (
        <div className="playlist-list">
          {jobs.map((j) => (
            <div key={j.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">⬇</div>
                <div style={{ flex: 1 }}>
                  <div className="playlist-name">
                    {j.title || j.url}
                    <span className={`status-badge status-${j.status}`}>
                      {t(`downloads.status${j.status[0].toUpperCase()}${j.status.slice(1)}`)}
                    </span>
                  </div>
                  <div className="muted">
                    {j.target_path}
                    {j.format && ` · ${j.format}`}
                    {j.interval_secs > 0 && ` · ${t('downloads.every', { hours: j.interval_secs / 3600 })}`}
                  </div>
                  <div className="progress">
                    <div className="progress-bar" style={{ width: `${j.progress || 0}%` }} />
                  </div>
                  <div className="muted small">
                    {t('downloads.progress', { pct: Math.round(j.progress || 0) })}
                  </div>
                  {j.error && <div className="form-error small">{j.error}</div>}
                </div>
              </div>
              <div className="detail-actions">
                {(j.status === 'queued' || j.status === 'downloading') && (
                  <button className="btn ghost" onClick={() => cancel(j.id)}>
                    {t('downloads.cancel')}
                  </button>
                )}
                {(j.status === 'failed' || j.status === 'canceled') && (
                  <button className="btn ghost" onClick={() => retry(j.id)}>
                    {t('downloads.retry')}
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
