import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function TraktAdmin() {
  const { t } = useTranslation();
  const [status, setStatus] = useState(null);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/trakt/status').then(setStatus).catch((e) => setErr(e.message));
  }
  useEffect(load, []);

  async function sync() {
    setBusy(true);
    setErr('');
    setMsg('');
    try {
      const d = await api('/admin/trakt/sync', { method: 'POST' });
      setMsg(t('trakt.synced', { count: d.pushed }));
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
      <div className="card admin-tools">
        <div className="admin-tools-head">{t('trakt.title')}</div>
        <p className="muted small">{t('trakt.hint')}</p>
        {status && (
          <div className="field-row">
            <span className={`status-badge ${status.configured ? 'status-approved' : 'status-pending'}`}>
              {status.configured ? t('trakt.configured') : t('trakt.notConfigured')}
            </span>
            <button className="btn small primary" disabled={busy || !status.configured} onClick={sync}>
              {busy ? t('trakt.syncing') : t('trakt.syncNow')}
            </button>
          </div>
        )}
        {status?.last_sync && (
          <div className="field-row">
            <span className="muted small">
              {t('trakt.lastSync')}: {new Date(status.last_sync.synced_at).toLocaleString()} · {status.last_sync.item_count}
            </span>
            {status.last_sync.error && <span className="form-error small">{status.last_sync.error}</span>}
          </div>
        )}
      </div>
    </div>
  );
}
