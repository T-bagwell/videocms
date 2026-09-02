import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function ScrapersAdmin() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [name, setName] = useState('');
  const [kind, setKind] = useState('url');
  const [command, setCommand] = useState('');
  const [url, setUrl] = useState('');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/scrapers').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
  }
  useEffect(load, []);

  async function run(fn, okKey) {
    setBusy(true);
    setErr('');
    setMsg('');
    try {
      await fn();
      setMsg(t(okKey));
      load();
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  async function register(e) {
    e.preventDefault();
    await run(async () => {
      await api('/admin/scrapers', {
        method: 'POST',
        body: { name, kind, command, url },
      });
      setName('');
      setCommand('');
      setUrl('');
    }, 'scrapers.registered');
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <p className="muted small">{t('scrapers.hint')}</p>
      <form className="card admin-tools" onSubmit={register}>
        <div className="admin-tools-head">{t('scrapers.register')}</div>
        <div className="field-row">
          <input placeholder={t('scrapers.name')} value={name} onChange={(e) => setName(e.target.value)} required />
          <select value={kind} onChange={(e) => setKind(e.target.value)}>
            <option value="url">URL</option>
            <option value="command">Command</option>
          </select>
          {kind === 'url' ? (
            <input placeholder={t('scrapers.url')} value={url} onChange={(e) => setUrl(e.target.value)} required />
          ) : (
            <input placeholder={t('scrapers.command')} value={command} onChange={(e) => setCommand(e.target.value)} required />
          )}
          <button className="btn small primary" disabled={busy}>{t('scrapers.register')}</button>
        </div>
      </form>

      <div className="card">
        {items.length === 0 && <div className="empty">{t('scrapers.empty')}</div>}
        {items.map((s) => (
          <div key={s.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{s.name}</b>
              <span className="muted small">
                {[s.kind, s.command || s.url].filter(Boolean).join(' · ')}
              </span>
              <span className={`status-badge ${s.enabled ? 'status-approved' : 'status-pending'}`}>
                {s.enabled ? t('scrapers.enabled') : t('scrapers.disabled')}
              </span>
            </div>
            <div className="version-actions">
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/scrapers/${s.id}`, { method: 'PATCH', body: { enabled: !s.enabled } });
                }, 'scrapers.updated')}
              >
                {s.enabled ? t('scrapers.disable') : t('scrapers.enable')}
              </button>
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/scrapers/${s.id}`, { method: 'DELETE' });
                }, 'scrapers.deleted')}
              >
                {t('common.remove')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
