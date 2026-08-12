import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

// ShareModal creates, lists and revokes public share links for a video,
// series or playlist. kind is the API collection: 'videos' | 'series' | 'playlists'.
export default function ShareModal({ kind, id, onClose }) {
  const { t } = useTranslation();
  const [hours, setHours] = useState(168);
  const [password, setPassword] = useState('');
  const [domains, setDomains] = useState('');
  const [shares, setShares] = useState([]);
  const [created, setCreated] = useState('');
  const [copied, setCopied] = useState(false);
  const [err, setErr] = useState('');

  useEffect(() => {
    api(`/${kind}/${id}/shares`).then((d) => setShares(d.items)).catch(() => {});
  }, [kind, id]);

  async function createShareLink(e) {
    e.preventDefault();
    setErr('');
    try {
      const d = await api(`/${kind}/${id}/share`, {
        method: 'POST',
        body: {
          hours: Number(hours) || 168,
          password: password || undefined,
          domains: domains
            .split(',')
            .map((s) => s.trim())
            .filter(Boolean),
        },
      });
      setCreated(`${window.location.origin}${d.url}`);
      const list = await api(`/${kind}/${id}/shares`);
      setShares(list.items);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function revokeShare(token) {
    setErr('');
    try {
      await api(`/share/${token}`, { method: 'DELETE' });
      setShares((prev) => prev.filter((s) => s.token !== token));
      if (created && created.endsWith(token)) setCreated('');
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function copyShare() {
    try {
      await navigator.clipboard.writeText(created);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard unavailable; the link is still visible to copy manually
    }
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{t('video.shareTitle')}</h3>
        <form onSubmit={createShareLink} className="inline-form">
          <input
            type="number"
            min="1"
            max="8760"
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            placeholder={t('video.shareHours')}
          />
          <button className="btn small primary">{t('video.shareCreate')}</button>
        </form>
        <label className="share-password-field">
          {t('video.sharePassword')}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder={t('video.sharePasswordPlaceholder')}
          />
        </label>
        <label className="share-password-field">
          {t('video.shareDomains')}
          <input
            value={domains}
            onChange={(e) => setDomains(e.target.value)}
            placeholder={t('video.shareDomainsPlaceholder')}
          />
        </label>
        {created && (
          <div className="share-link-row">
            <code className="share-link">{created}</code>
            <button className="btn small" onClick={copyShare}>
              {copied ? t('video.shareCopied') : t('video.shareCopy')}
            </button>
          </div>
        )}
        {err && <div className="form-error">{err}</div>}
        {shares.length > 0 && (
          <div className="share-list">
            <h4>{t('video.shareList')}</h4>
            {shares.map((s) => (
              <div key={s.token} className="share-row">
                <span className="muted">
                  {t('video.shareExpires', {
                    date: new Date(s.expires_at).toLocaleString(),
                  })}
                </span>
                <button className="btn small ghost" onClick={() => revokeShare(s.token)}>
                  {t('video.shareRevoke')}
                </button>
              </div>
            ))}
          </div>
        )}
        <button className="btn ghost" onClick={onClose}>
          {t('common.close')}
        </button>
      </div>
    </div>
  );
}
