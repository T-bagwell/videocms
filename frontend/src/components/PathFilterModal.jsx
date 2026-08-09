import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function PathFilterModal({ onClose, onChanged }) {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [path, setPath] = useState('');
  const [err, setErr] = useState('');

  function refresh() {
    api('/users/me/hidden-paths')
      .then((d) => setItems(d.items))
      .catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  async function add(e) {
    e.preventDefault();
    setErr('');
    try {
      await api('/users/me/hidden-paths', { method: 'POST', body: { path } });
      setPath('');
      refresh();
      onChanged?.();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function remove(id) {
    try {
      await api(`/users/me/hidden-paths/${id}`, { method: 'DELETE' });
      refresh();
      onChanged?.();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal path-picker" onClick={(e) => e.stopPropagation()}>
        <h3>{t('series.pathFilter')}</h3>
        <p className="muted hint">{t('series.pathFilterHint')}</p>
        <form className="inline-form" onSubmit={add}>
          <input
            placeholder={t('series.pathPlaceholder')}
            value={path}
            onChange={(e) => setPath(e.target.value)}
          />
          <button className="btn small primary">{t('series.addPath')}</button>
        </form>
        {err && <div className="form-error small">{err}</div>}
        {items.length === 0 ? (
          <div className="picker-empty">{t('series.noHiddenPaths')}</div>
        ) : (
          <div className="picker-list">
            {items.map((p) => (
              <div key={p.id} className="picker-dir hidden-path-row">
                <span className="mono ellipsis">{p.path}</span>
                <button className="btn small danger-ghost" onClick={() => remove(p.id)}>
                  {t('series.removePath')}
                </button>
              </div>
            ))}
          </div>
        )}
        <div className="modal-actions">
          <button className="btn ghost" onClick={onClose}>
            {t('common.close')}
          </button>
        </div>
      </div>
    </div>
  );
}

