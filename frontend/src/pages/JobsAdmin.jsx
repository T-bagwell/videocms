import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import { fmtBytes } from '../i18n';

export default function JobsAdmin() {
  const { t } = useTranslation();
  const [jobs, setJobs] = useState([]);
  const [disk, setDisk] = useState({ disk_free: 0, disk_total: 0 });
  const [err, setErr] = useState('');

  const refresh = useCallback(() => {
    api('/admin/jobs').then((d) => setJobs(d.items || [])).catch(() => {});
    api('/admin/system').then(setDisk).catch(() => {});
  }, []);
  useEffect(() => {
    refresh();
  }, [refresh]);
  const active = jobs.some((j) => ['scanning', 'uploading', 'queued', 'downloading', 'live', 'starting'].includes(j.status));
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(refresh, 3000);
    return () => clearInterval(timer);
  }, [active, refresh]);

  async function act(kind, id, action) {
    setErr('');
    const map = {
      'scan-cancel': { method: 'POST', url: `/libraries/${id}/scan/cancel` },
      'download-cancel': { method: 'DELETE', url: `/downloads/${id}` },
      'download-retry': { method: 'POST', url: `/downloads/${id}/retry` },
      'upload-cancel': { method: 'DELETE', url: `/uploads/${id}` },
      'live-start': { method: 'POST', url: `/live/${id}/start` },
      'live-stop': { method: 'POST', url: `/live/${id}/stop` },
    };
    const op = map[`${kind}-${action}`];
    if (!op) return;
    try {
      await api(op.url, { method: op.method });
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  return (
    <div>
      {err && <div className="form-error">{err}</div>}
      <div className="stats-grid">
        <div className="card stat">
          <div className="stat-num">{fmtBytes(disk.disk_free)}</div>
          <div>{t('admin.jobsDiskFree')}</div>
        </div>
        <div className="card stat">
          <div className="stat-num">{fmtBytes(disk.disk_total)}</div>
          <div>{t('admin.jobsDiskTotal')}</div>
        </div>
      </div>
      {jobs.length === 0 ? (
        <div className="empty">{t('admin.jobsEmpty')}</div>
      ) : (
        <div className="playlist-list">
          {jobs.map((j) => (
            <div key={`${j.kind}-${j.id}`} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">⚙</div>
                <div style={{ flex: 1 }}>
                  <div className="playlist-name">
                    <span className="status-badge status-idle">{t(`admin.jobsKind${j.kind[0].toUpperCase()}${j.kind.slice(1)}`)}</span>
                    <span className="ellipsis">{j.title}</span>
                    <span className={`status-badge status-${j.status === 'error' || j.status === 'failed' ? 'error' : j.status}`}>
                      {j.status}
                    </span>
                  </div>
                  {(j.kind === 'download' || j.status === 'uploading') && (
                    <div className="progress">
                      <div className="progress-bar" style={{ width: `${j.progress || 0}%` }} />
                    </div>
                  )}
                  {j.error && <div className="form-error small">{j.error}</div>}
                </div>
              </div>
              <div className="detail-actions">
                {j.kind === 'scan' && j.status === 'scanning' && (
                  <button className="btn ghost" onClick={() => act('scan', j.id, 'cancel')}>{t('admin.stopScan')}</button>
                )}
                {j.kind === 'download' && (j.status === 'queued' || j.status === 'downloading') && (
                  <button className="btn ghost" onClick={() => act('download', j.id, 'cancel')}>{t('admin.jobsCancel')}</button>
                )}
                {j.kind === 'download' && (j.status === 'failed' || j.status === 'canceled') && (
                  <button className="btn ghost" onClick={() => act('download', j.id, 'retry')}>{t('admin.jobsRetry')}</button>
                )}
                {j.kind === 'upload' && j.status === 'uploading' && (
                  <button className="btn ghost" onClick={() => act('upload', j.id, 'cancel')}>{t('admin.jobsCancel')}</button>
                )}
                {j.kind === 'live' && (j.status === 'live' || j.status === 'starting') && (
                  <button className="btn ghost" onClick={() => act('live', j.id, 'stop')}>{t('admin.jobsStop')}</button>
                )}
                {j.kind === 'live' && (j.status === 'idle' || j.status === 'offline') && (
                  <button className="btn ghost" onClick={() => act('live', j.id, 'start')}>{t('admin.jobsStart')}</button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
