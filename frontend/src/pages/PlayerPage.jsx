import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

const BROWSER_PLAYABLE = ['.mp4', '.m4v', '.webm', '.mov', '.ogv'];

export default function PlayerPage() {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const videoRef = useRef(null);
  const savedRef = useRef(null);
  const hlsRef = useRef(null);
  const offsetRef = useRef(0);
  const lastRestartRef = useRef(0);
  const [video, setVideo] = useState(null);
  const [queue, setQueue] = useState([]);
  const [queueTitle, setQueueTitle] = useState('');
  const [useTranscode, setUseTranscode] = useState(false);
  const [transcoding, setTranscoding] = useState(false);
  const [hlsErr, setHlsErr] = useState('');
  const [err, setErr] = useState('');

  const saveProgress = useCallback(() => {
    const el = videoRef.current;
    if (!el || !el.duration || !id) return;
    const position = Math.max(0, offsetRef.current + (el.currentTime || 0));
    const duration = offsetRef.current + el.duration;
    api('/users/me/progress', {
      method: 'PUT',
      body: {
        video_id: id,
        position_sec: position,
        duration_sec: duration,
      },
    }).catch(() => {});
  }, [id]);

  useEffect(() => {
    api(`/videos/${id}`).then(setVideo).catch((e) => setErr(e.message));

    const playlistId = searchParams.get('playlist');
    const seriesId = searchParams.get('series');
    setQueue([]);
    setQueueTitle('');
    if (playlistId) {
      api(`/playlists/${playlistId}`)
        .then((d) => {
          setQueue(d.items.map((i) => i.video));
          setQueueTitle(t('player.fromPlaylist', { name: d.playlist.name }));
        })
        .catch(() => {});
    } else if (seriesId) {
      api(`/series/${seriesId}`)
        .then((d) => {
          setQueue(d.items);
          setQueueTitle(t('player.fromSeries', { name: d.series.name }));
        })
        .catch(() => {});
    }
  }, [id, searchParams, t]);

  const startTranscode = useCallback(async () => {
    if (!video) return;
    const Hls = (await import('hls.js')).default;
    const start = offsetRef.current > 0 ? Math.floor(offsetRef.current) : 0;
    const hls = new Hls({
      startPosition: 0,
      maxBufferLength: 30,
      maxMaxBufferLength: 90,
    });
    hlsRef.current = hls;
    setTranscoding(true);
    setHlsErr('');
    const url = mediaUrl(`/videos/${id}/hls/playlist.m3u8`) + `&start=${start}`;

    hls.on(Hls.Events.MANIFEST_PARSED, () => setTranscoding(false));
    hls.on(Hls.Events.ERROR, (_event, data) => {
      if (!data.fatal) return;
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
        hls.startLoad();
        return;
      }
      setHlsErr(t('player.transcodeFailed', { detail: data.details || 'unknown' }));
      setTranscoding(false);
    });
    hls.attachMedia(videoRef.current);
    hls.loadSource(url);
  }, [id, video, t]);

  useEffect(() => {
    if (!video) return;
    const ext = video.filename?.match(/\.[^.]+$/)?.[0]?.toLowerCase() || '';
    const shouldTranscode = searchParams.get('transcode') === '1' || !BROWSER_PLAYABLE.includes(ext);
    setUseTranscode(shouldTranscode);
    if (!shouldTranscode) return;
    const initialStart = video.progress_sec > 5 ? Math.floor(video.progress_sec) : 0;
    offsetRef.current = initialStart;
    startTranscode();
    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
        hlsRef.current = null;
      }
    };
  }, [video, searchParams, startTranscode]);

  useEffect(() => {
    savedRef.current = null;
  }, [id]);

  function playNext() {
    if (!queue.length) return;
    const idx = queue.findIndex((v) => v.id === id);
    const next = queue[idx + 1];
    if (next) {
      const qp = queueParam();
      navigate(qp ? `/player/${next.id}?${qp}` : `/player/${next.id}`);
    }
  }

  function queueParam() {
    const p = searchParams.get('playlist');
    if (p) return `playlist=${p}`;
    const s = searchParams.get('series');
    if (s) return `series=${s}`;
    return '';
  }

  function onLoadedMetadata() {
    const el = videoRef.current;
    if (!useTranscode && el && video?.progress_sec > 5 && video.progress_sec < video.duration_sec * 0.95) {
      el.currentTime = video.progress_sec;
    }
  }

  function restartTranscode(newStart) {
    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }
    offsetRef.current = newStart;
    startTranscode();
  }

  function onSeeking() {
    if (!useTranscode || !hlsRef.current) return;
    const el = videoRef.current;
    if (!el) return;
    const target = el.currentTime;
    let bufferStart = Infinity;
    let bufferEnd = 0;
    for (let i = 0; i < el.buffered.length; i++) {
      bufferStart = Math.min(bufferStart, el.buffered.start(i));
      bufferEnd = Math.max(bufferEnd, el.buffered.end(i));
    }
    if (target >= bufferStart - 3 && target <= bufferEnd + 3) return;
    if (Date.now() - lastRestartRef.current < 3000) return;
    lastRestartRef.current = Date.now();
    restartTranscode(Math.max(0, Math.floor(offsetRef.current + target)));
  }

  function onError() {
    // native playback failed; offer transcode fallback
    if (!useTranscode) {
      setHlsErr(t('player.transcodeFailed', { detail: 'unsupported format' }));
    }
  }

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!video) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  const queueIdx = queue.findIndex((v) => v.id === id);
  const streamUrl = useTranscode ? undefined : mediaUrl(`/videos/${id}/stream`);

  return (
    <div className="container player-page">
      <div className="player-head">
        <Link to={`/video/${video.id}`} className="btn ghost">{t('player.backToDetail')}</Link>
        <div>
          <h1>{video.title}</h1>
          {queueTitle && <p className="muted">{queueTitle}</p>}
        </div>
      </div>

      <video
        key={id}
        ref={videoRef}
        className="player"
        controls
        autoPlay
        src={streamUrl}
        poster={video.has_poster ? mediaUrl(`/videos/${id}/poster`) : undefined}
        onLoadedMetadata={onLoadedMetadata}
        onSeeking={onSeeking}
        onTimeUpdate={() => {
          const el = videoRef.current;
          if (!el) return;
          if (!savedRef.current || el.currentTime - savedRef.current > 5) {
            savedRef.current = el.currentTime;
            saveProgress();
          }
        }}
        onPause={saveProgress}
        onEnded={() => {
          saveProgress();
          playNext();
        }}
        onError={onError}
      />

      {transcoding && (
        <div className="banner info">{t('player.transcoding')}</div>
      )}

      {hlsErr && (
        <div className="banner warn">
          {hlsErr}{' '}
          {!useTranscode && (
            <button className="btn small" onClick={() => navigate(`/player/${id}?transcode=1`)}>
              {t('player.transcodePlay')}
            </button>
          )}
          <a className="btn small" href={mediaUrl(`/videos/${video.id}/download`)}>
            {t('common.download')}
          </a>
        </div>
      )}

      {queue.length > 1 && (
        <div className="queue">
          <h3>{t('player.queue', { count: queue.length })}</h3>
          {queue.map((v, i) => (
            <button
              key={v.id}
              className={`queue-item ${v.id === id ? 'current' : ''}`}
              onClick={() =>
                v.id !== id &&
                (() => {
                  const qp = queueParam();
                  navigate(qp ? `/player/${v.id}?${qp}` : `/player/${v.id}`);
                })()
              }
            >
              <span className="queue-idx">{i + 1}</span>
              <span className="queue-title">{v.title}</span>
              {i === queueIdx && <span className="queue-now">{t('player.nowPlaying')}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
