import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function Invites() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [count, setCount] = useState('1');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/invites').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
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
      <div className="card admin-tools">
        <div className="admin-tools-head">{t('invites.title')}</div>
        <div className="field-row">
          <input
            type="number"
            min={1}
            max={50}
            value={count}
            onChange={(e) => setCount(e.target.value)}
          />
          <button
            className="btn small primary"
            disabled={busy}
            onClick={() => run(async () => {
              await api('/admin/invites', { method: 'POST', body: { count: Number(count) || 1 } });
            }, 'invites.generated')}
          >
            {t('invites.generate')}
          </button>
        </div>
      </div>
      <div className="card">
        {items.length === 0 && <div className="empty">{t('invites.empty')}</div>}
        {items.map((it) => (
          <div key={it.id} className="playlist-row">
            <div className="playlist-item-main">
              <code>{it.code}</code>
              <span className="muted small">
                {it.used_user ? `${t('invites.usedBy')}: ${it.used_user}` : t('invites.unused')}
              </span>
            </div>
            <button
              className="btn small ghost"
              disabled={busy}
              onClick={() => run(async () => {
                await api(`/admin/invites/${it.id}`, { method: 'DELETE' });
              }, 'invites.revoked')}
            >
              {t('invites.revoke')}
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
