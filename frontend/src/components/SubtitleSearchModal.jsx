import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function SubtitleSearchModal({ video, onClose, onDownloaded }) {
  const { t } = useTranslation();
  const [lang, setLang] = useState('');
  const [items, setItems] = useState(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState('');

  async function search(e) {
    e.preventDefault();
    setBusy(true);
    setErr('');
    setItems(null);
    try {
      const d = await api(`/videos/${video.id}/subtitles/search`, {
        method: 'POST',
        body: { language: lang.trim() },
      });
      setItems(d.items || []);
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  async function download(c) {
    setBusy(true);
    setErr('');
    try {
      const d = await api(`/videos/${video.id}/subtitles/download`, {
        method: 'POST',
        body: { file_id: c.id },
      });
      onDownloaded?.(d);
      onClose();
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{t('video.subtitleSearchTitle')}</h3>
        {err && <div className="form-error">{err}</div>}
        <form className="inline-form" onSubmit={search}>
          <input
            placeholder={t('video.subtitleSearchLang')}
            value={lang}
            onChange={(e) => setLang(e.target.value)}
          />
          <button className="btn primary" disabled={busy}>
            {busy ? t('common.loading') : t('video.subtitleSearchBtn')}
          </button>
        </form>
        {items && items.length === 0 && <div className="empty">{t('video.subtitleSearchEmpty')}</div>}
        {items && items.length > 0 && (
          <div className="playlist-list">
            {items.map((c) => (
              <div key={c.id} className="card playlist-card">
                <div className="playlist-main">
                  <div>
                    <div className="playlist-name">{c.title || t('video.subtitleTrack')}</div>
                    <div className="muted">{c.language}</div>
                  </div>
                  {c.provider && (
                    <span className="status-badge status-idle">{c.provider}</span>
                  )}
                </div>
                <div className="detail-actions">
                  <button className="btn ghost" disabled={busy} onClick={() => download(c)}>
                    {t('video.subtitleDownload')}
                  </button>
                </div>
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
