import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function QualityProfiles() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [name, setName] = useState('');
  const [minHeight, setMinHeight] = useState('');
  const [maxHeight, setMaxHeight] = useState('');
  const [codec, setCodec] = useState('');
  const [active, setActive] = useState(false);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/quality-profiles').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
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
        <div className="admin-tools-head">{t('quality.add')}</div>
        <div className="field-row">
          <input placeholder={t('quality.name')} value={name} onChange={(e) => setName(e.target.value)} />
          <input
            type="number"
            placeholder={t('quality.minHeight')}
            value={minHeight}
            onChange={(e) => setMinHeight(e.target.value)}
          />
          <input
            type="number"
            placeholder={t('quality.maxHeight')}
            value={maxHeight}
            onChange={(e) => setMaxHeight(e.target.value)}
          />
          <input placeholder={t('quality.codec')} value={codec} onChange={(e) => setCodec(e.target.value)} />
          <label className="scrape-force">
            <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} />
            {t('quality.active')}
          </label>
          <button
            className="btn small primary"
            disabled={busy || !name.trim()}
            onClick={() => run(async () => {
              await api('/admin/quality-profiles', {
                method: 'POST',
                body: {
                  name,
                  min_height: Number(minHeight) || 0,
                  max_height: Number(maxHeight) || 0,
                  preferred_codec: codec,
                  active,
                },
              });
              setName(''); setMinHeight(''); setMaxHeight(''); setCodec(''); setActive(false);
            }, 'quality.saved')}
          >
            {t('quality.add')}
          </button>
        </div>
      </div>

      <div className="card">
        <div className="admin-tools-head">
          {t('quality.profiles')}
          <button className="btn small ghost" disabled={busy} onClick={() => run(
            () => api('/admin/quality-profiles/apply', { method: 'POST' }),
            'quality.applied',
          )}>
            {t('quality.applyNow')}
          </button>
        </div>
        {items.length === 0 && <div className="empty">{t('quality.empty')}</div>}
        {items.map((p) => (
          <div key={p.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{p.name}</b>
              <span className="muted small">
                {[p.min_height > 0 ? `${p.min_height}p+` : '', p.max_height > 0 ? `≤${p.max_height}p` : '', p.preferred_codec]
                  .filter(Boolean)
                  .join(' · ') || t('quality.any')}
              </span>
              {p.active && <span className="status-badge status-approved">{t('quality.active')}</span>}
            </div>
            <div className="version-actions">
              {!p.active && (
                <button
                  className="btn small primary"
                  disabled={busy}
                  onClick={() => run(
                    () => api(`/admin/quality-profiles/${p.id}/active`, { method: 'POST' }),
                    'quality.activated',
                  )}
                >
                  {t('quality.activate')}
                </button>
              )}
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(
                  () => api(`/admin/quality-profiles/${p.id}`, { method: 'DELETE' }),
                  'quality.deleted',
                )}
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
