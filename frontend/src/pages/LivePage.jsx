import { useEffect, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

export default function LivePage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const videoRef = useRef(null);
  const hlsRef = useRef(null);
  const [stream, setStream] = useState(null);
  const [err, setErr] = useState('');
  const [messages, setMessages] = useState([]);
  const [body, setBody] = useState('');
  const [msgErr, setMsgErr] = useState('');

  useEffect(() => {
    api(`/live/${id}`).then(setStream).catch((e) => setErr(e.message));
    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [id]);

  useEffect(() => {
    if (!stream) return;
    let destroyed = false;
    (async () => {
      const Hls = (await import('hls.js')).default;
      if (destroyed) return;
      const hls = new Hls({ liveDurationInfinity: true, maxLiveSyncPlaybackRate: 1 });
      hlsRef.current = hls;
      hls.loadSource(mediaUrl(`/live/${stream.id}/hls/index.m3u8`));
      hls.attachMedia(videoRef.current);
    })();
    return () => {
      destroyed = true;
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [stream]);

  useEffect(() => {
    if (!stream) return;
    let last = '';
    const poll = async () => {
      try {
        const d = await api(`/live/${stream.id}/chat${last ? `?after=${last}` : ''}`);
        if (d.items?.length) {
          last = d.items[d.items.length - 1].id;
          setMessages((prev) => [...prev, ...d.items].slice(-200));
        }
      } catch {
        // room may be gone; keep polling
      }
    };
    poll();
    const timer = setInterval(poll, 3000);
    return () => clearInterval(timer);
  }, [stream]);

  async function send(e) {
    e.preventDefault();
    setMsgErr('');
    try {
      await api(`/live/${stream.id}/chat`, { method: 'POST', body: { body } });
      setBody('');
    } catch (e2) {
      setMsgErr(e2.message);
    }
  }

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!stream) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  const statusKey = `status${stream.status[0].toUpperCase()}${stream.status.slice(1)}`;
  return (
    <div className="container">
      <div className="player-head">
        <Link to="/" className="btn ghost">{t('nav.home')}</Link>
        <div>
          <h1>{stream.title}</h1>
          <p className="muted">
            <span className={`status-badge status-${stream.status}`}>{t(`live.${statusKey}`)}</span>
          </p>
        </div>
      </div>
      <video ref={videoRef} className="player" controls autoPlay />
      {stream.status !== 'live' && <div className="banner info">{t('live.offlineHint')}</div>}
      <div className="chat-box">
        <h3>{t('live.chat')}</h3>
        <div className="chat-messages">
          {messages.length === 0 && <div className="empty">{t('live.chatEmpty')}</div>}
          {messages.map((m) => (
            <div key={m.id} className="chat-message">
              <b>{m.username || '?'}</b> {m.body}
            </div>
          ))}
        </div>
        <form className="inline-form" onSubmit={send}>
          <input
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder={t('live.chatPlaceholder')}
            maxLength={500}
          />
          <button className="btn primary">{t('live.send')}</button>
        </form>
        {msgErr && <div className="form-error">{msgErr}</div>}
      </div>
    </div>
  );
}
