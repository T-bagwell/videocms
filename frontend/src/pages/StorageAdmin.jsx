import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function StorageAdmin() {
  const { t } = useTranslation();
  const [pools, setPools] = useState([]);
  const [form, setForm] = useState({ name: '', type: 'local', mount_path: '', config: '{}', readonly: false });
  const [editingId, setEditingId] = useState('');
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  function refresh() {
    api('/admin/storage-pools').then((d) => setPools(d.items || [])).catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  function resetForm() {
    setForm({ name: '', type: 'local', mount_path: '', config: '{}', readonly: false });
    setEditingId('');
  }

  async function save(e) {
    e.preventDefault();
    setErr('');
    let cfg = form.config;
    try {
      cfg = JSON.parse(cfg || '{}');
    } catch {
      setErr(t('admin.storageConfigInvalid'));
      return;
    }
    const body = { name: form.name, type: form.type, mount_path: form.mount_path, config: cfg, readonly: form.readonly };
    try {
      if (editingId) {
        await api(`/admin/storage-pools/${editingId}`, { method: 'PATCH', body });
      } else {
        await api('/admin/storage-pools', { method: 'POST', body });
      }
      setMsg(t('admin.storageSaved'));
      resetForm();
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function remove(id) {
    if (!window.confirm(t('admin.storageDeleteConfirm'))) return;
    try {
      await api(`/admin/storage-pools/${id}`, { method: 'DELETE' });
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  function edit(p) {
    setEditingId(p.id);
    setForm({
      name: p.name,
      type: p.type,
      mount_path: p.mount_path,
      config: JSON.stringify(p.config || {}, null, 2),
      readonly: p.readonly,
    });
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card inline-form" onSubmit={save}>
        <input placeholder={t('admin.storageName')} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        <select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
          <option value="local">{t('admin.storageTypeLocal')}</option>
          <option value="s3">{t('admin.storageTypeS3')}</option>
          <option value="sftp">{t('admin.storageTypeSftp')}</option>
        </select>
        <input placeholder={t('admin.storageMount')} value={form.mount_path} onChange={(e) => setForm({ ...form, mount_path: e.target.value })} required />
        <input placeholder={t('admin.storageConfig')} value={form.config} onChange={(e) => setForm({ ...form, config: e.target.value })} />
        <label className="scrape-force">
          <input type="checkbox" checked={form.readonly} onChange={(e) => setForm({ ...form, readonly: e.target.checked })} />
          {t('admin.storageReadonly')}
        </label>
        <button className="btn primary">{editingId ? t('admin.storageSave') : t('admin.storageAdd')}</button>
        {editingId && <button type="button" className="btn ghost" onClick={resetForm}>{t('common.cancel')}</button>}
      </form>
      <p className="muted hint">{t('admin.storageHint')}</p>
      {pools.length === 0 ? (
        <div className="empty">{t('admin.storageEmpty')}</div>
      ) : (
        <div className="playlist-list">
          {pools.map((p) => (
            <div key={p.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">💾</div>
                <div>
                  <div className="playlist-name">
                    {p.name}
                    <span className="status-badge status-idle">{p.type}</span>
                    {p.readonly && <span className="status-badge status-error">{t('admin.storageReadonly')}</span>}
                  </div>
                  <div className="muted mono">{p.mount_path}</div>
                </div>
              </div>
              <div className="detail-actions">
                <button className="btn ghost" onClick={() => edit(p)}>{t('common.edit')}</button>
                <button className="btn ghost danger-ghost" onClick={() => remove(p.id)}>{t('admin.delete')}</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
