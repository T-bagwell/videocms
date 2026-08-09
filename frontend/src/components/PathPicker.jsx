import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import { fmtBytes } from '../i18n';

export default function PathPicker({ initialPath = '', onPick, onClose }) {
  const { t } = useTranslation();
  const [current, setCurrent] = useState('');
  const [parent, setParent] = useState('');
  const [home, setHome] = useState('');
  const [dirs, setDirs] = useState([]);
  const [free, setFree] = useState(0);
  const [err, setErr] = useState('');
  const [loading, setLoading] = useState(false);

  function load(path) {
    setLoading(true);
    setErr('');
    api(`/admin/paths?path=${encodeURIComponent(path || '')}`)
      .then((d) => {
        setCurrent(d.current || '');
        setParent(d.parent || '');
        setHome(d.home || '');
        setDirs(d.dirs || []);
        setFree(d.free_bytes || 0);
        if (d.error) setErr(d.error);
      })
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false));
  }

  useEffect(() => {
    load(initialPath);
  }, [initialPath]);

  const segments = current.split('/').filter(Boolean);
  const crumbs = segments.map((seg, i) => ({
    name: seg,
    path: '/' + segments.slice(0, i + 1).join('/'),
  }));

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal path-picker" onClick={(e) => e.stopPropagation()}>
        <h3>{t('picker.title')}</h3>
        <div className="picker-toolbar">
          <button className="btn small" onClick={() => home && load(home)}>
            {t('picker.home')}
          </button>
          <button className="btn small" onClick={() => load('/')}>
            {t('picker.root')}
          </button>
          {parent && (
            <button className="btn small" onClick={() => load(parent)}>
              {t('picker.parent')}
            </button>
          )}
          <span className="picker-free">{t('picker.freeSpace', { size: fmtBytes(free) })}</span>
        </div>

        <div className="picker-path mono" title={current}>
          <span className="crumb" onClick={() => load('/')}>/</span>
          {crumbs.map((c) => (
            <span key={c.path}>
              <span className="sep">/</span>
              <span className="crumb" onClick={() => load(c.path)}>{c.name}</span>
            </span>
          ))}
        </div>

        <div className="picker-list">
          {loading && <div className="picker-empty">{t('common.loading')}</div>}
          {err && <div className="form-error small picker-error">{err}</div>}
          {!loading && !err && dirs.length === 0 && (
            <div className="picker-empty">{t('picker.empty')}</div>
          )}
          {!loading &&
            dirs.map((d) => (
              <button key={d.path} className="picker-dir" onClick={() => load(d.path)}>
                <span className="picker-dir-icon">📁</span> {d.name}
              </button>
            ))}
        </div>

        <div className="picker-current mono" title={current}>
          {t('picker.current', { path: current || '…' })}
        </div>
        <div className="modal-actions">
          <button className="btn primary" onClick={() => onPick(current)} disabled={!current}>
            {t('picker.select')}
          </button>
          <button className="btn ghost" onClick={onClose}>
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </div>
  );
}
