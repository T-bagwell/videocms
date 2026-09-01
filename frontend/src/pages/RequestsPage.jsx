import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

function statusKey(status) {
  return `requests.status${status.charAt(0).toUpperCase()}${status.slice(1)}`;
}

export default function RequestsPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [title, setTitle] = useState('');
  const [year, setYear] = useState('');
  const [mediaType, setMediaType] = useState('movie');
  const [notes, setNotes] = useState('');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/requests').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
  }
  useEffect(load, []);

  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setErr('');
    setMsg('');
    try {
      await api('/requests', {
        method: 'POST',
        body: { title, year: Number(year) || 0, media_type: mediaType, notes },
      });
      setTitle('');
      setYear('');
      setNotes('');
      setMsg(t('requests.submitted'));
      load();
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container">
      <h1>{t('nav.requests')}</h1>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card admin-tools" onSubmit={submit}>
        <div className="admin-tools-head">{t('requests.submit')}</div>
        <div className="field-row">
          <input placeholder={t('requests.titleField')} value={title} onChange={(e) => setTitle(e.target.value)} required />
          <input
            type="number"
            min={1900}
            max={2100}
            placeholder={t('requests.year')}
            value={year}
            onChange={(e) => setYear(e.target.value)}
          />
          <select value={mediaType} onChange={(e) => setMediaType(e.target.value)}>
            <option value="movie">{t('requests.typeMovie')}</option>
            <option value="tv">{t('requests.typeTv')}</option>
          </select>
          <button className="btn small primary" disabled={busy}>{t('requests.submit')}</button>
        </div>
        <div className="field-row">
          <input placeholder={t('requests.notes')} value={notes} onChange={(e) => setNotes(e.target.value)} />
        </div>
      </form>

      <div className="card">
        <h3>{t('requests.myRequests')}</h3>
        {items.length === 0 && <div className="empty">{t('requests.empty')}</div>}
        {items.map((r) => (
          <div key={r.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{r.title}</b>
              <span className="muted small">
                {[r.year > 0 ? r.year : '', r.media_type, r.notes].filter(Boolean).join(' · ')}
              </span>
            </div>
            <span className={`status-badge status-${r.status}`}>{t(statusKey(r.status))}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function AdminRequests() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [inputs, setInputs] = useState({});

  function load() {
    api('/requests/all').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
  }
  useEffect(load, []);

  async function decide(id, status) {
    setErr('');
    setMsg('');
    try {
      const cfg = inputs[id] || {};
      await api(`/requests/${id}/decide`, {
        method: 'POST',
        body: {
          status,
          download_url: status === 'approved' ? cfg.download_url || '' : '',
          target_path: status === 'approved' ? cfg.target_path || '' : '',
        },
      });
      setMsg(t('requests.decided'));
      load();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <div className="card">
        <h3>{t('requests.all')}</h3>
        {items.length === 0 && <div className="empty">{t('requests.empty')}</div>}
        {items.map((r) => (
          <div key={r.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{r.title}</b>
              <span className="muted small">
                {[r.username, r.year > 0 ? r.year : '', r.media_type, r.notes].filter(Boolean).join(' · ')}
              </span>
              {r.status === 'pending' && (
                <div className="field-row">
                  <input
                    placeholder={t('requests.downloadUrl')}
                    value={inputs[r.id]?.download_url || ''}
                    onChange={(e) => setInputs((p) => ({ ...p, [r.id]: { ...p[r.id], download_url: e.target.value } }))}
                  />
                  <input
                    placeholder={t('requests.targetPath')}
                    value={inputs[r.id]?.target_path || ''}
                    onChange={(e) => setInputs((p) => ({ ...p, [r.id]: { ...p[r.id], target_path: e.target.value } }))}
                  />
                </div>
              )}
            </div>
            <div className="version-actions">
              <span className={`status-badge status-${r.status}`}>{t(statusKey(r.status))}</span>
              {r.status === 'pending' && (
                <>
                  <button className="btn small primary" onClick={() => decide(r.id, 'approved')}>
                    {t('requests.approve')}
                  </button>
                  <button className="btn small ghost" onClick={() => decide(r.id, 'rejected')}>
                    {t('requests.reject')}
                  </button>
                </>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
