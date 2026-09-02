import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function TranscodeAdmin() {
  const { t } = useTranslation();
  const [jobs, setJobs] = useState([]);
  const [videos, setVideos] = useState([]);
  const [videoId, setVideoId] = useState('');
  const [priority, setPriority] = useState('5');
  const [workers, setWorkers] = useState(1);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/admin/transcode/jobs')
      .then((d) => {
        setJobs(d.items || []);
        setWorkers(d.workers || 1);
      })
      .catch((e) => setErr(e.message));
    api('/videos?page_size=100')
      .then((d) => setVideos(d.items || []))
      .catch(() => {});
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

  async function enqueue(e) {
    e.preventDefault();
    await run(async () => {
      await api('/admin/transcode/queue', {
        method: 'POST',
        body: { video_id: videoId, priority: Number(priority) || 5 },
      });
      setVideoId('');
      setPriority('5');
    }, 'transcode.queued');
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <p className="muted small">{t('transcode.hint', { workers })}</p>
      <form className="card admin-tools" onSubmit={enqueue}>
        <div className="admin-tools-head">{t('transcode.enqueue')}</div>
        <div className="field-row">
          <select value={videoId} onChange={(e) => setVideoId(e.target.value)} required>
            <option value="">{t('transcode.video')}</option>
            {videos.map((v) => (
              <option key={v.id} value={v.id}>{v.title}</option>
            ))}
          </select>
          <input
            type="number"
            min={1}
            max={10}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
          />
          <button className="btn small primary" disabled={busy}>{t('transcode.enqueue')}</button>
        </div>
      </form>

      <div className="card">
        <h3>{t('transcode.jobs')}</h3>
        {jobs.length === 0 && <div className="empty">{t('transcode.empty')}</div>}
        {jobs.map((j) => (
          <div key={j.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{j.video_title}</b>
              <span className="muted small">
                {[t('transcode.priority', { n: j.priority }), new Date(j.created_at).toLocaleString(), j.error]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
            <div className="version-actions">
              <span className={`status-badge status-${j.status === 'done' ? 'approved' : j.status === 'failed' ? 'rejected' : 'pending'}`}>
                {t(`transcode.status${j.status.charAt(0).toUpperCase()}${j.status.slice(1)}`)}
              </span>
              {(j.status === 'queued' || j.status === 'running') && (
                <button
                  className="btn small ghost"
                  disabled={busy}
                  onClick={() => run(async () => {
                    await api(`/admin/transcode/jobs/${j.id}`, { method: 'DELETE' });
                  }, 'transcode.cancelled')}
                >
                  {t('transcode.cancel')}
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
