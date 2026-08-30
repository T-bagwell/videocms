import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import PathPicker from '../components/PathPicker.jsx';
import UploadManager from '../components/UploadManager.jsx';
import { fmtBytes } from '../i18n';

export default function UploadsAdmin() {
  const { t } = useTranslation();
  const [target, setTarget] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [sessions, setSessions] = useState([]);
  const [err, setErr] = useState('');
  const uploaderRef = useRef(null);
  const fileInputRef = useRef(null);

  const refreshSessions = useCallback(() => {
    api('/uploads')
      .then((d) => setSessions((d.items || []).filter((s) => s.status === 'uploading')))
      .catch(() => {});
  }, []);

  useEffect(() => {
    refreshSessions();
  }, [refreshSessions]);

  useEffect(() => {
    if (sessions.length === 0) return;
    const timer = setInterval(refreshSessions, 3000);
    return () => clearInterval(timer);
  }, [sessions, refreshSessions]);

  async function cancelSession(id) {
    setErr('');
    try {
      await api(`/uploads/${id}`, { method: 'DELETE' });
      refreshSessions();
    } catch (e) {
      setErr(e.message);
    }
  }

  return (
    <div>
      {err && <div className="form-error">{err}</div>}
      <div className="card">
        <h3>{t('uploads.target')}</h3>
        <div className="path-field">
          <input readOnly value={target} placeholder={t('uploads.noTarget')} />
          <button type="button" className="btn" onClick={() => setPickerOpen(true)}>
            {t('admin.browse')}
          </button>
        </div>
        <p className="muted hint">{t('uploads.targetHint')}</p>
        {pickerOpen && (
          <PathPicker
            initialPath={target}
            onPick={(p) => {
              setTarget(p);
              setPickerOpen(false);
            }}
            onClose={() => setPickerOpen(false)}
          />
        )}
      </div>

      {target ? (
        <>
          <div
            className="card dropzone"
            onClick={() => fileInputRef.current?.click()}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              uploaderRef.current?.addFiles(e.dataTransfer.files);
            }}
          >
            <input
              ref={fileInputRef}
              type="file"
              multiple
              hidden
              onChange={(e) => {
                uploaderRef.current?.addFiles(e.target.files);
                e.target.value = '';
              }}
            />
            <p>{t('uploads.dropHint')}</p>
          </div>
          <h3>{t('uploads.queue')}</h3>
          <UploadManager ref={uploaderRef} targetPath={target} />
        </>
      ) : (
        <div className="empty">{t('uploads.chooseTargetFirst')}</div>
      )}

      <h3>{t('uploads.serverSessions')}</h3>
      {sessions.length === 0 ? (
        <div className="empty">{t('uploads.noSessions')}</div>
      ) : (
        <div className="playlist-list">
          {sessions.map((s) => (
            <div key={s.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">⬆</div>
                <div>
                  <div className="playlist-name">{s.filename}</div>
                  <div className="muted">
                    {fmtBytes(s.received_sum || 0)} / {fmtBytes(s.total_size || 0)} · {s.target_path}
                  </div>
                </div>
              </div>
              <div className="detail-actions">
                <button className="btn ghost" onClick={() => cancelSession(s.id)}>
                  {t('uploads.cancel')}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
