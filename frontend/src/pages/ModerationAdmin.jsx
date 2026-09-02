import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function ModerationAdmin() {
  const { t } = useTranslation();
  const [reports, setReports] = useState([]);
  const [users, setUsers] = useState([]);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/reports').then((d) => setReports(d.items || [])).catch((e) => setErr(e.message));
    api('/users').then((d) => setUsers(d.items || [])).catch(() => {});
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

      <div className="card">
        <h3>{t('moderation.reports')}</h3>
        {reports.length === 0 && <div className="empty">{t('moderation.noReports')}</div>}
        {reports.map((rep) => (
          <div key={rep.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{rep.video_title}</b>
              <span className="muted small">
                {[rep.reporter, rep.reason, rep.details, new Date(rep.created_at).toLocaleString()]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
            <div className="version-actions">
              <span className={`status-badge status-${rep.status === 'pending' ? 'pending' : 'idle'}`}>
                {t(`moderation.status${rep.status.charAt(0).toUpperCase()}${rep.status.slice(1)}`)}
              </span>
              {rep.status === 'pending' && (
                <>
                  <button
                    className="btn small primary"
                    disabled={busy}
                    onClick={() => run(async () => {
                      await api(`/admin/reports/${rep.id}/decide`, { method: 'POST', body: { status: 'reviewed' } });
                    }, 'moderation.updated')}
                  >
                    {t('moderation.review')}
                  </button>
                  <button
                    className="btn small ghost"
                    disabled={busy}
                    onClick={() => run(async () => {
                      await api(`/admin/reports/${rep.id}/decide`, { method: 'POST', body: { status: 'dismissed' } });
                    }, 'moderation.updated')}
                  >
                    {t('moderation.dismiss')}
                  </button>
                </>
              )}
            </div>
          </div>
        ))}
      </div>

      <div className="card">
        <h3>{t('moderation.users')}</h3>
        {users.map((u) => (
          <div key={u.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{u.username}</b>
              <span className="muted small">
                {[u.display_name, u.role, u.muted ? t('moderation.muted') : '', u.global_blocked ? t('moderation.blocked') : '']
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
            <div className="version-actions">
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/users/${u.id}/moderation`, { method: 'PATCH', body: { muted: !u.muted } });
                }, 'moderation.updated')}
              >
                {u.muted ? t('moderation.unmute') : t('moderation.mute')}
              </button>
              <button
                className="btn small ghost"
                disabled={busy}
                onClick={() => run(async () => {
                  await api(`/admin/users/${u.id}/moderation`, { method: 'PATCH', body: { global_blocked: !u.global_blocked } });
                }, 'moderation.updated')}
              >
                {u.global_blocked ? t('moderation.unblock') : t('moderation.block')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
