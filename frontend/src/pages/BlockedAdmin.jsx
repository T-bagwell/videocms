import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function BlockedAdmin() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [title, setTitle] = useState('');
  const [preview, setPreview] = useState([]);
  const [previewing, setPreviewing] = useState('');
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  function refresh() {
    api('/admin/blocked-titles')
      .then((d) => setItems(d.items))
      .catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  async function add(e) {
    e.preventDefault();
    setErr('');
    setMsg('');
    const value = title.trim();
    if (!value) return;
    try {
      await api('/admin/blocked-titles', { method: 'POST', body: { title: value } });
      setTitle('');
      setPreview([]);
      setPreviewing('');
      setMsg(t('admin.blockAdded', { title: value }));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function previewSearch(e) {
    e.preventDefault();
    setErr('');
    const value = previewing.trim();
    if (!value) return;
    try {
      const d = await api(`/videos?page_size=20&include_blocked=1&q=${encodeURIComponent(value)}`);
      setPreview(d.items);
      setMsg('');
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function unblock(b) {
    setErr('');
    try {
      await api(`/admin/blocked-titles/${b.id}`, { method: 'DELETE' });
      setMsg(t('admin.blockRemoved', { title: b.title }));
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
          placeholder={t('admin.blockPlaceholder')}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
        />
        <button className="btn primary">{t('admin.blockAdd')}</button>
      </form>
      <p className="muted hint">{t('admin.blockHint')}</p>

      <form className="search-form" onSubmit={previewSearch}>
        <input
          className="search-input"
          placeholder={t('admin.blockPreviewPlaceholder')}
          value={previewing}
          onChange={(e) => setPreviewing(e.target.value)}
        />
        <button className="btn">{t('common.search')}</button>
      </form>

      {preview.length > 0 && (
        <div className="admin-video-list">
          {preview.map((v) => (
            <div key={v.id} className="card admin-video-row">
              <div className="mono muted" style={{ flex: 1, minWidth: 0 }}>
                <div className="ellipsis">{v.title}</div>
                <div className="small-muted">{v.filename}</div>
              </div>
              {v.blocked && <span className="status-badge status-error">{t('admin.blockedBadge')}</span>}
            </div>
          ))}
        </div>
      )}

      <h3>{t('admin.blockList')}</h3>
      {items.length === 0 ? (
        <div className="empty">{t('admin.blockEmpty')}</div>
      ) : (
        <div className="playlist-list">
          {items.map((b) => (
            <div key={b.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">🚫</div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div className="playlist-name ellipsis">{b.title}</div>
                  <div className="muted">
                    {t('admin.blockMatches', { count: b.match_count })}
                    {' · '}
                    {new Date(b.created_at).toLocaleString()}
                  </div>
                </div>
              </div>
              <div className="playlist-item-actions">
                <button className="btn small danger-ghost" onClick={() => unblock(b)}>
                  {t('admin.blockUnblock')}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
