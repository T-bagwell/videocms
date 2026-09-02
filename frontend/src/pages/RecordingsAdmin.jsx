import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

export default function RecordingsAdmin() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [channels, setChannels] = useState([]);
  const [tuners, setTuners] = useState([]);
  const [channelId, setChannelId] = useState('');
  const [title, setTitle] = useState('');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);
  const [playing, setPlaying] = useState(null);

  function load() {
    api('/admin/recordings').then((d) => setItems(d.items || [])).catch((e) => setErr(e.message));
    api('/iptv/channels').then((d) => setChannels(d.items || [])).catch(() => {});
    api('/admin/tuners').then((d) => setTuners(d.items || [])).catch(() => {});
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

  async function schedule(e) {
    e.preventDefault();
    await run(async () => {
      await api('/admin/recordings', {
        method: 'POST',
        body: {
          channel_id: channelId,
          title,
          start_utc: new Date(start).toISOString(),
          end_utc: new Date(end).toISOString(),
        },
      });
      setTitle('');
      setStart('');
      setEnd('');
    }, 'recordings.scheduled');
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}

      <div className="card admin-tools">
        <div className="admin-tools-head">{t('recordings.schedule')}</div>
        <form onSubmit={schedule}>
          <div className="field-row">
            <select value={channelId} onChange={(e) => setChannelId(e.target.value)} required>
              <option value="">{t('recordings.channel')}</option>
              {channels.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
            <input placeholder={t('recordings.title')} value={title} onChange={(e) => setTitle(e.target.value)} required />
          </div>
          <div className="field-row">
            <input type="datetime-local" value={start} onChange={(e) => setStart(e.target.value)} required />
            <input type="datetime-local" value={end} onChange={(e) => setEnd(e.target.value)} required />
            <button className="btn small primary" disabled={busy}>{t('recordings.schedule')}</button>
          </div>
        </form>
      </div>

      <div className="card admin-tools">
        <div className="admin-tools-head">
          {t('recordings.tuners')}
          <button className="btn small ghost" disabled={busy} onClick={() => run(
            () => api('/admin/tuners/scan', { method: 'POST' }),
            'recordings.scanned',
          )}>
            {t('recordings.scan')}
          </button>
        </div>
        {tuners.length === 0 && <div className="empty">{t('recordings.noTuners')}</div>}
        {tuners.map((u) => (
          <div key={u} className="playlist-row"><code>{u}</code></div>
        ))}
      </div>

      <div className="card">
        <h3>{t('recordings.list')}</h3>
        {items.length === 0 && <div className="empty">{t('recordings.empty')}</div>}
        {items.map((rec) => (
          <div key={rec.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{rec.title}</b>
              <span className="muted small">
                {[rec.channel, new Date(rec.start_utc).toLocaleString(), new Date(rec.end_utc).toLocaleString(), rec.error]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
            <div className="version-actions">
              <span className={`status-badge status-${rec.status}`}>{t(`recordings.status${rec.status.charAt(0).toUpperCase()}${rec.status.slice(1)}`)}</span>
              {rec.status === 'done' && (
                <button className="btn small primary" onClick={() => setPlaying(rec)}>
                  {t('recordings.play')}
                </button>
              )}
              <button
                className="btn small ghost"
                onClick={() => run(async () => {
                  await api(`/admin/recordings/${rec.id}`, { method: 'DELETE' });
                }, 'recordings.deleted')}
              >
                {t('common.remove')}
              </button>
            </div>
          </div>
        ))}
      </div>
      {playing && (
        <div className="modal-backdrop" onClick={() => setPlaying(null)}>
          <div className="modal modal-wide" onClick={(e) => e.stopPropagation()}>
            <h3>{playing.title}</h3>
            <video
              className="featurette-player"
              src={mediaUrl(`/iptv/recordings/${playing.id}/stream`)}
              controls
              autoPlay
            />
            <div className="modal-actions">
              <button className="btn ghost" onClick={() => setPlaying(null)}>{t('common.close')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
