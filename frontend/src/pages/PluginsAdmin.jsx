import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

const DEFAULT_EVENTS = ['scan', 'download', 'comment', 'favorite'];

export default function PluginsAdmin() {
  const { t } = useTranslation();
  const [directory, setDirectory] = useState([]);
  const [items, setItems] = useState([]);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/plugins/directory').then((d) => setDirectory(d.items || [])).catch(() => {});
    api('/admin/plugins').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
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

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}

      <div className="card">
        <h3>{t('plugins.directory')}</h3>
        {directory.length === 0 && <div className="empty">{t('plugins.emptyDirectory')}</div>}
        {directory.map((p) => (
          <div key={p.name} className="playlist-row">
            <div className="playlist-item-main">
              <b>{p.name}</b>
              <span className="muted small">{[p.description, p.kind].filter(Boolean).join(' · ')}</span>
            </div>
            <button
              className="btn small primary"
              disabled={busy}
              onClick={() => run(async () => {
                await api('/admin/plugins', {
                  method: 'POST',
                  body: {
                    name: p.name,
                    description: p.description,
                    install_url: p.install_url || '',
                    kind: p.kind,
                    events: DEFAULT_EVENTS,
                  },
                });
              }, 'plugins.installOk')}
            >
              {t('plugins.install')}
            </button>
          </div>
        ))}
      </div>

      <div className="card">
        <h3>{t('plugins.installed')}</h3>
        {items.length === 0 && <div className="empty">{t('plugins.empty')}</div>}
        {items.map((p) => (
          <div key={p.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{p.name}</b>
              <span className="muted small">
                {[p.description, p.kind, p.events?.join(', ')].filter(Boolean).join(' · ')}
              </span>
              <span className={`status-badge ${p.enabled ? 'status-approved' : 'status-pending'}`}>
                {p.enabled ? t('plugins.enabled') : t('plugins.disabled')}
              </span>
            </div>
            <div className="version-actions">
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/plugins/${p.id}`, { method: 'PATCH', body: { enabled: !p.enabled } });
                }, 'plugins.updated')}
              >
                {p.enabled ? t('plugins.disable') : t('plugins.enable')}
              </button>
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/plugins/${p.id}`, { method: 'DELETE' });
                }, 'plugins.uninstalled')}
              >
                {t('plugins.uninstall')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
