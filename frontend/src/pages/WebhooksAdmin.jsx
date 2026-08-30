import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function WebhooksAdmin() {
  const { t } = useTranslation();
  const [hooks, setHooks] = useState([]);
  const [form, setForm] = useState({ url: '', secret: '', events: '', active: true });
  const [editingId, setEditingId] = useState('');
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  function refresh() {
    api('/admin/webhooks').then((d) => setHooks(d.items || [])).catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  function resetForm() {
    setForm({ url: '', secret: '', events: '', active: true });
    setEditingId('');
  }

  async function save(e) {
    e.preventDefault();
    setErr('');
    const body = {
      url: form.url.trim(),
      secret: form.secret,
      events: form.events.split(',').map((s) => s.trim()).filter(Boolean),
      active: form.active,
    };
    try {
      if (editingId) {
        await api(`/admin/webhooks/${editingId}`, { method: 'PATCH', body });
      } else {
        await api('/admin/webhooks', { method: 'POST', body });
      }
      setMsg(t('admin.webhookSaved'));
      resetForm();
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function remove(id) {
    try {
      await api(`/admin/webhooks/${id}`, { method: 'DELETE' });
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  function edit(h) {
    setEditingId(h.id);
    setForm({ url: h.url, secret: h.secret || '', events: (h.events || []).join(', '), active: h.active });
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card inline-form" onSubmit={save}>
        <input placeholder={t('admin.webhookUrl')} value={form.url} onChange={(e) => setForm({ ...form, url: e.target.value })} required />
        <input placeholder={t('admin.webhookSecret')} value={form.secret} onChange={(e) => setForm({ ...form, secret: e.target.value })} />
        <input placeholder={t('admin.webhookEvents')} value={form.events} onChange={(e) => setForm({ ...form, events: e.target.value })} />
        <label className="scrape-force">
          <input type="checkbox" checked={form.active} onChange={(e) => setForm({ ...form, active: e.target.checked })} />
          {t('admin.webhookActive')}
        </label>
        <button className="btn primary">{editingId ? t('admin.webhookSave') : t('admin.webhookAdd')}</button>
        {editingId && <button type="button" className="btn ghost" onClick={resetForm}>{t('common.cancel')}</button>}
      </form>
      <p className="muted hint">{t('admin.webhookHint')}</p>
      {hooks.length === 0 ? (
        <div className="empty">{t('admin.webhookEmpty')}</div>
      ) : (
        <div className="playlist-list">
          {hooks.map((h) => (
            <div key={h.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">⚡</div>
                <div>
                  <div className="playlist-name">
                    {h.url}
                    {!h.active && <span className="status-badge status-error">{t('admin.webhookInactive')}</span>}
                  </div>
                  <div className="muted small">{(h.events || []).join(', ') || t('admin.webhookAllEvents')}</div>
                </div>
              </div>
              <div className="detail-actions">
                <button className="btn ghost" onClick={() => edit(h)}>{t('common.edit')}</button>
                <button className="btn ghost danger-ghost" onClick={() => remove(h.id)}>{t('admin.delete')}</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
