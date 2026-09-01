import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, apiBaseUrl, mediaUrl } from '../api.js';
import DownloadDialog from '../components/DownloadDialog.jsx';
import { DownloadIcon } from '../components/Icons.jsx';
import { fmtDuration } from '../i18n';
import JASSUB from 'jassub';

// jassub assets are copied from node_modules to public/jassub by
// scripts/copy-jassub.mjs before build/dev.
const JASS_BASE = `${import.meta.env.BASE_URL}jassub/`;
const JASSUB_WORKER = `${JASS_BASE}jassub-worker.js`;
const JASSUB_WASM = `${JASS_BASE}jassub-worker.wasm`;
const JASSUB_MODERN_WASM = `${JASS_BASE}jassub-worker-modern.wasm`;
const JASSUB_FONT = `${JASS_BASE}default.woff2`;

const BROWSER_PLAYABLE = ['.mp4', '.m4v', '.webm', '.mov', '.ogv'];

// Load the Google Cast sender SDK once; resolves when cast APIs are ready.
function loadCastSDK() {
  return new Promise((resolve, reject) => {
    if (window.chrome?.cast?.isAvailable) {
      resolve();
      return;
    }
    if (document.querySelector('script[src*="cast_sender.js"]')) {
      const iv = setInterval(() => {
        if (window.chrome?.cast?.isAvailable) {
          clearInterval(iv);
          resolve();
        }
      }, 200);
      return;
    }
    window.__onGCastApiAvailable = (available) => {
      if (available) resolve();
    };
    const s = document.createElement('script');
    s.src = 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1';
    s.async = true;
    s.onerror = () => reject(new Error('failed to load cast sdk'));
    document.head.appendChild(s);
  });
}

export default function PlayerPage() {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { t } = useTranslation();
  // activeId lets us switch episodes without unmounting the <video> element,
  // so fullscreen survives auto-advance. It follows the route id.
  const [activeId, setActiveId] = useState(id);
  const videoRef = useRef(null);
  const savedRef = useRef(null);
  const hlsRef = useRef(null);
  const tracksRef = useRef([]);
  const offsetRef = useRef(0);
  const lastRestartRef = useRef(0);
  const [video, setVideo] = useState(null);
  const [queue, setQueue] = useState([]);
  const [queueTitle, setQueueTitle] = useState('');
  const [useTranscode, setUseTranscode] = useState(false);
  const [transcoding, setTranscoding] = useState(false);
  const [hlsErr, setHlsErr] = useState('');
  const [err, setErr] = useState('');
  const [levels, setLevels] = useState([]);
  const [quality, setQuality] = useState('auto');
  const [tracks, setTracks] = useState([]);
  const [subtitleTracks, setSubtitleTracks] = useState([]);
  const [subtitleIdx, setSubtitleIdx] = useState(-1);
  const [audioTracks, setAudioTracks] = useState([]);
  const [audioIdx, setAudioIdx] = useState(-1);
  const [subtitleOffsetMs, setSubtitleOffsetMs] = useState(0);
  const [thumbMeta, setThumbMeta] = useState(null);
  const [hover, setHover] = useState(null);
  const [assIdx, setAssIdx] = useState('');
  const assRef = useRef(null);
  const [watchRoom, setWatchRoom] = useState(null);
  const [watchModal, setWatchModal] = useState(false);
  const [showDownload, setShowDownload] = useState(false);
  const [watchJoinId, setWatchJoinId] = useState('');
  const [watchJoinToken, setWatchJoinToken] = useState('');
  const watchRoomRef = useRef(null);
  watchRoomRef.current = watchRoom;
  const [hasAirPlay, setHasAirPlay] = useState(false);
  const [skip, setSkip] = useState({ intro: null, credits: null });
  const [skipDraft, setSkipDraft] = useState(null);
  const [chapters, setChapters] = useState([]);
  const [hasCast, setHasCast] = useState(false);
  const [casting, setCasting] = useState(false);

  useEffect(() => {
    setActiveId(id);
  }, [id]);

  useEffect(() => {
    let stopped = false;
    loadCastSDK()
      .then(() => {
        if (!stopped && window.chrome?.cast?.isAvailable) setHasCast(true);
      })
      .catch(() => {});
    return () => { stopped = true; };
  }, []);

  useEffect(() => {
    if (assRef.current) {
      assRef.current.destroy().catch(() => {});
      assRef.current = null;
    }
    setAssIdx('');
  }, [activeId]);

  const saveProgress = useCallback(() => {
    const el = videoRef.current;
    if (!el || !el.duration || !activeId) return;
    const position = Math.max(0, offsetRef.current + (el.currentTime || 0));
    const duration = offsetRef.current + el.duration;
    api('/users/me/progress', {
      method: 'PUT',
      body: {
        video_id: activeId,
        position_sec: position,
        duration_sec: duration,
      },
    }).catch(() => {});
  }, [activeId]);

  useEffect(() => {
    api(`/videos/${activeId}`).then(setVideo).catch((e) => setErr(e.message));
    api(`/videos/${activeId}/subtitle-tracks`)
      .then((d) => {
        setTracks(d.items);
        tracksRef.current = d.items;
      })
      .catch(() => {
        setTracks([]);
        tracksRef.current = [];
      });
    api(`/users/me/subtitle-offset?video_id=${activeId}`)
      .then((d) => setSubtitleOffsetMs(d.offset_ms || 0))
      .catch(() => setSubtitleOffsetMs(0));
    api(`/videos/${activeId}/thumbnails`)
      .then(setThumbMeta)
      .catch(() => setThumbMeta(null));
    api(`/videos/${activeId}/skip-intervals`)
      .then((d) => setSkip({ intro: d.intro || null, credits: d.credits || null }))
      .catch(() => {});
    api(`/videos/${activeId}/chapters`)
      .then((d) => setChapters(d.items || []))
      .catch(() => setChapters([]));

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
  }, [activeId, searchParams, t]);

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
    setLevels([]);
    setAudioTracks([]);
    setAudioIdx(-1);
    const url = mediaUrl(`/videos/${activeId}/hls/playlist.m3u8`) + `&start=${start}`;

    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      setTranscoding(false);
      if (hls.levels && hls.levels.length > 1) {
        setLevels(
          hls.levels.map((l, i) => ({
            i,
            label: l.height ? `${l.height}p` : `${Math.round(l.bitrate / 1000)}k`,
          })),
        );
      }
    });
    hls.on(Hls.Events.LEVEL_SWITCHED, (_event, d) => {
      setQuality(String(d.level));
    });
    hls.on(Hls.Events.SUBTITLE_TRACKS_UPDATED, (_event, d) => {
      const list = d.subtitleTracks || [];
      setSubtitleTracks(
        list.map((tr, i) => ({
          i,
          id: tr.id,
          label: tr.name || tr.lang || `${t('player.subtitles')} ${i + 1}`,
        })),
      );
      const pref = tracksRef.current.find((t) => t.is_active);
      if (pref && list.length > 0) {
        const idx = list.findIndex((tr) => String(tr.id).includes(pref.id));
        if (idx >= 0 && hls.subtitleTrack !== idx) {
          hls.subtitleTrack = idx;
          setSubtitleIdx(idx);
          return;
        }
      }
      setSubtitleIdx(hls.subtitleTrack);
    });
    hls.on(Hls.Events.SUBTITLE_TRACK_SWITCH, (_event, d) => {
      setSubtitleIdx(d.id ?? -1);
    });
    hls.on(Hls.Events.AUDIO_TRACKS_UPDATED, (_event, d) => {
      const list = d.audioTracks || [];
      setAudioTracks(
        list.map((tr, i) => ({
          i,
          id: tr.id,
          label: tr.name || tr.lang || `${t('player.audioTrack')} ${i + 1}`,
        })),
      );
      if (list.length > 1 && hls.audioTrack !== 0) hls.audioTrack = 0;
      setAudioIdx(hls.audioTrack ?? -1);
    });
    hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_event, d) => {
      setAudioIdx(d.id ?? -1);
    });
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
  }, [activeId, video, t]);

  function changeQuality(q) {
    setQuality(q);
    const hls = hlsRef.current;
    if (!hls || !hls.levels || !hls.levels.length) return;
    hls.currentLevel = q === 'auto' ? -1 : Number(q);
  }

  function changeSubtitle(i) {
    const hls = hlsRef.current;
    if (!hls) return;
    hls.subtitleTrack = i;
    setSubtitleIdx(i);
  }

  function changeAudio(i) {
    const hls = hlsRef.current;
    if (!hls) return;
    hls.audioTrack = i;
    setAudioIdx(i);
  }

  function adjustSubtitleOffset(delta) {
    const next = Math.max(-300000, Math.min(300000, subtitleOffsetMs + delta));
    setSubtitleOffsetMs(next);
    if (assIdx) void setAssTrack(assIdx);
    api('/users/me/subtitle-offset', {
      method: 'PUT',
      body: { video_id: activeId, offset_ms: next },
    }).catch(() => {});
  }

  function resetSubtitleOffset() {
    setSubtitleOffsetMs(0);
    api(`/users/me/subtitle-offset?video_id=${activeId}`, { method: 'DELETE' }).catch(() => {});
  }

  function onStripMove(e) {
    const el = videoRef.current;
    const rect = e.currentTarget.getBoundingClientRect();
    const pct = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const dur = el?.duration || video?.duration_sec || 0;
    const time = pct * dur;
    const frame = thumbMeta && dur > 0
      ? Math.min(thumbMeta.count, Math.floor(time / thumbMeta.interval_sec) + 1)
      : 1;
    setHover({ pct: pct * 100, time, frame });
  }

  function onStripClick() {
    if (!hover || !videoRef.current) return;
    videoRef.current.currentTime = hover.time;
  }

  async function setAssTrack(trackId) {
    if (assRef.current) {
      try {
        await assRef.current.destroy();
      } catch {
        // instance may already be destroyed
      }
      assRef.current = null;
    }
    setAssIdx(trackId || '');
    if (trackId && hlsRef.current) hlsRef.current.subtitleTrack = -1;
    if (!trackId) return;
    const tr = tracksRef.current.find((x) => x.id === trackId);
    if (!tr) return;
    try {
      const res = await fetch(mediaUrl(`/videos/${activeId}/subtitles/${trackId}`));
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const content = await res.text();
      assRef.current = new JASSUB({
        video: videoRef.current,
        subContent: content,
        workerUrl: JASSUB_WORKER,
        wasmUrl: JASSUB_WASM,
        modernWasmUrl: JASSUB_MODERN_WASM,
        fonts: [JASSUB_FONT],
        defaultFont: JASSUB_FONT,
        timeOffset: subtitleOffsetMs / 1000,
        onDemandRender: true,
      });
    } catch {
      setAssIdx('');
    }
  }

  function markSkip(kind) {
    const el = videoRef.current;
    if (!el) return;
    const now = el.currentTime || 0;
    if (skipDraft?.kind === kind) {
      const end = Math.max(now, skipDraft.start + 1);
      api(`/videos/${activeId}/skip-interval`, {
        method: 'PUT',
        body: { kind, start_sec: skipDraft.start, end_sec: end },
      })
        .then(() => setSkip((prev) => ({ ...prev, [kind]: { start_sec: skipDraft.start, end_sec: end } })))
        .catch(() => {});
      setSkipDraft(null);
    } else {
      setSkipDraft({ kind, start: now });
    }
  }

  function skipTo(kind) {
    const el = videoRef.current;
    const interval = kind === 'intro' ? skip.intro : skip.credits;
    if (!el || !interval) return;
    el.currentTime = interval.end_sec;
  }

  function seekToChapter(ch) {
    const el = videoRef.current;
    if (el && isFinite(el.duration)) {
      el.currentTime = ch.start_sec;
      el.play().catch(() => {});
    }
  }

  function clearSkip(kind) {
    api(`/videos/${activeId}/skip-interval?kind=${kind}`, { method: 'DELETE' })
      .then(() => setSkip((prev) => ({ ...prev, [kind]: null })))
      .catch(() => {});
  }

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
  }, [activeId]);

  function switchEpisode(nextId) {
    if (nextId === activeId) return;
    saveProgress();
    if (hlsRef.current) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }
    setActiveId(nextId);
    setHlsErr('');
    setTranscoding(false);
    const qp = queueParam();
    navigate(qp ? `/player/${nextId}?${qp}` : `/player/${nextId}`);
  }

  function playNext() {
    if (!queue.length) return;
    const idx = queue.findIndex((v) => v.id === activeId);
    const next = queue[idx + 1];
    if (next) switchEpisode(next.id);
  }

  function queueParam() {
    const p = searchParams.get('playlist');
    if (p) return `playlist=${p}`;
    const s = searchParams.get('series');
    if (s) return `series=${s}`;
    return '';
  }

  // resume native playback from saved progress once the new episode's metadata
  // is available (guard against stale video data during episode switching)
  useEffect(() => {
    if (!video || video.id !== activeId || useTranscode) return;
    const el = videoRef.current;
    if (!el) return;
    const p = video.progress_sec;
    if (!(p > 5 && p < video.duration_sec * 0.95)) return;
    const apply = () => {
      if (el.readyState >= 1) el.currentTime = p;
    };
    if (el.readyState >= 1) {
      apply();
    } else {
      el.addEventListener('loadedmetadata', apply, { once: true });
      return () => el.removeEventListener('loadedmetadata', apply);
    }
  }, [video, activeId, useTranscode]);

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

  function publishWatch(playing) {
    const room = watchRoomRef.current;
    const el = videoRef.current;
    if (!room || !el) return;
    api(`/watch/rooms/${room.id}`, {
      method: 'PUT',
      body: { token: room.token, playing, position_sec: el.currentTime || 0 },
    }).catch(() => {});
  }

  useEffect(() => {
    if (!watchRoom) return;
    const timer = setInterval(async () => {
      const room = watchRoomRef.current;
      const el = videoRef.current;
      if (!room || !el) return;
      try {
        const d = await api(`/watch/rooms/${room.id}?token=${room.token}`);
        if (Math.abs((el.currentTime || 0) - d.position_sec) > 2) {
          el.currentTime = d.position_sec;
        }
        if (d.playing && el.paused) el.play().catch(() => {});
        if (!d.playing && !el.paused) el.pause();
      } catch {
        // room may have been deleted; keep polling
      }
    }, 2500);
    return () => clearInterval(timer);
  }, [watchRoom]);

  async function createWatchRoom() {
    try {
      const d = await api('/watch/rooms', { method: 'POST', body: { video_id: activeId } });
      setWatchRoom(d);
      setWatchModal(false);
    } catch (e) {
      setErr(e.message);
    }
  }

  async function joinWatchRoom(e) {
    e.preventDefault();
    try {
      const d = await api(`/watch/rooms/${watchJoinId.trim()}/join`, {
        method: 'POST',
        body: { token: watchJoinToken.trim() },
      });
      setWatchRoom(d);
      setWatchModal(false);
      setWatchJoinId('');
      setWatchJoinToken('');
    } catch (e2) {
      setErr(e2.message);
    }
  }

  function leaveWatchRoom() {
    setWatchRoom(null);
  }

  function castToAirPlay() {
    const el = videoRef.current;
    if (!el || typeof el.webkitShowPlaybackUI !== 'function') return;
    el.webkitShowPlaybackUI();
  }

  async function castToChromecast() {
    if (!window.cast?.framework) return;
    try {
      const ctx = window.cast.framework.CastContext.getInstance();
      ctx.setOptions({
        receiverApplicationId: 'CC1AD845',
        autoJoinPolicy: window.chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
      });
      await ctx.requestSession();
      const share = await api(`/videos/${activeId}/share`, {
        method: 'POST',
        body: { hours: 1 },
      });
      const base = `${window.location.origin}${apiBaseUrl()}`;
      const stream = `${base}/api/share/${share.token}/video/${activeId}/stream`;
      const mediaInfo = new window.chrome.cast.media.MediaInfo(stream, 'video/mp4');
      mediaInfo.metadata = new window.chrome.cast.media.GenericMediaMetadata();
      mediaInfo.metadata.title = video.title;
      if (video.has_poster) {
        mediaInfo.metadata.images = [{
          url: `${base}/api/share/${share.token}/video/${activeId}/poster`,
        }];
      }
      const session = ctx.getCurrentSession();
      if (!session) throw new Error('no cast session');
      await session.loadMedia(new window.chrome.cast.media.LoadRequest(mediaInfo));
      setCasting(true);
    } catch (e) {
      console.error('cast to chromecast failed', e);
      setCasting(false);
    }
  }

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!video) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  const queueIdx = queue.findIndex((v) => v.id === activeId);
  const streamUrl = useTranscode ? undefined : mediaUrl(`/videos/${activeId}/stream`);

  return (
    <div className="container player-page">
      <div className="player-head">
        <Link to={`/video/${video.id}`} className="btn ghost">{t('player.backToDetail')}</Link>
        <div>
          <h1>{video.title}</h1>
          {queueTitle && <p className="muted">{queueTitle}</p>}
          {watchRoom && <p className="muted">{t('player.watchSyncing')}</p>}
        </div>
        <div className="detail-actions">
          {hasAirPlay && (
            <button className="btn ghost" onClick={castToAirPlay}>
              {t('player.cast')}
            </button>
          )}
          {hasCast && (
            <button className="btn ghost" onClick={castToChromecast}>
              {casting ? t('player.casting') : t('player.castChromecast')}
            </button>
          )}
          {!watchRoom ? (
            <button className="btn ghost" onClick={() => setWatchModal(true)}>
              {t('player.watchTogether')}
            </button>
          ) : (
            <button className="btn ghost" onClick={leaveWatchRoom}>
              {t('player.watchLeave')}
            </button>
          )}
          <button className="btn ghost" onClick={() => setShowDownload(true)}>
            <DownloadIcon />
            {t('common.download')}
          </button>
        </div>
      </div>

      {showDownload && (
        <DownloadDialog video={video} onClose={() => setShowDownload(false)} />
      )}

      {watchModal && (
        <div className="modal-backdrop" onClick={() => setWatchModal(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t('player.watchTogether')}</h3>
            <div className="modal-actions">
              <button className="btn primary" onClick={createWatchRoom}>
                {t('player.watchCreate')}
              </button>
            </div>
            <form className="inline-form" onSubmit={joinWatchRoom}>
              <input
                placeholder={t('player.watchJoinId')}
                value={watchJoinId}
                onChange={(e) => setWatchJoinId(e.target.value)}
                required
              />
              <input
                placeholder={t('player.watchJoinToken')}
                value={watchJoinToken}
                onChange={(e) => setWatchJoinToken(e.target.value)}
                required
              />
              <button className="btn ghost">{t('player.watchJoin')}</button>
            </form>
            <div className="modal-actions">
              <button className="btn ghost" onClick={() => setWatchModal(false)}>
                {t('common.close')}
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="player-wrap">
        <video
          ref={videoRef}
          className="player"
          controls
          autoPlay
          src={streamUrl}
          poster={video.has_poster ? mediaUrl(`/videos/${activeId}/poster`) : undefined}
          onLoadedMetadata={(e) => {
            setHasAirPlay(typeof e.currentTarget.webkitShowPlaybackUI === 'function');
          }}
          onSeeking={onSeeking}
          onTimeUpdate={() => {
            const el = videoRef.current;
            if (!el) return;
            if (!savedRef.current || el.currentTime - savedRef.current > 5) {
              savedRef.current = el.currentTime;
              saveProgress();
            }
            publishWatch(true);
          }}
          onPause={saveProgress}
          onPlay={() => publishWatch(true)}
          onEnded={() => {
            saveProgress();
            playNext();
          }}
          onError={onError}
        >
          {!useTranscode &&
            tracks
              .filter((tr) => tr.format !== 'ass' && tr.format !== 'ssa')
              .map((tr) => (
                <track
                  key={`${tr.id}-${subtitleOffsetMs}`}
                  kind="subtitles"
                  srcLang={tr.lang || undefined}
                  label={tr.title || tr.lang || t('player.subtitles')}
                  src={mediaUrl(
                    `/videos/${activeId}/subtitles/${tr.id}${subtitleOffsetMs ? `?offset_ms=${subtitleOffsetMs}` : ''}`,
                  )}
                  default={tr.is_active && !assIdx}
                />
              ))}
        </video>
      </div>

      {thumbMeta && (
        <div
          className="preview-strip"
          onMouseMove={onStripMove}
          onMouseLeave={() => setHover(null)}
          onClick={onStripClick}
        >
          {chapters.length > 0 &&
            video?.duration_sec > 0 &&
            chapters.map((ch) => (
              <span
                key={ch.id}
                className="chapter-tick"
                style={{ left: `${Math.min(99.5, (ch.start_sec / video.duration_sec) * 100)}%` }}
                title={ch.title || `${fmtDuration(ch.start_sec)}`}
              />
            ))}
          {hover && (
            <div className="preview-tip" style={{ left: `${hover.pct}%` }}>
              <img
                src={mediaUrl(`/videos/${activeId}/thumbnails/${hover.frame}`)}
                alt=""
                width={thumbMeta.width}
                height={thumbMeta.height}
              />
              <span>{fmtDuration(hover.time)}</span>
            </div>
          )}
        </div>
      )}

      {chapters.length > 0 && (
        <div className="player-tools chapters-bar">
          <span className="muted">{t('player.chaptersLabel')}</span>
          <div className="chapters-list">
            {chapters.map((ch) => (
              <button
                key={ch.id}
                className="btn small ghost chapter-chip"
                onClick={() => seekToChapter(ch)}
              >
                <span className="chapter-time">{fmtDuration(ch.start_sec)}</span>
                {ch.title || `#${ch.position + 1}`}
              </button>
            ))}
          </div>
        </div>
      )}

      {!useTranscode && tracks.length > 0 && (
        <div className="player-tools subtitle-sync">
          <span className="muted">
            {t('player.subtitleSync')}: {subtitleOffsetMs >= 0 ? '+' : ''}
            {subtitleOffsetMs / 1000}s
          </span>
          <button className="btn small" onClick={() => adjustSubtitleOffset(-500)}>
            {t('player.subtitleEarlier')}
          </button>
          <button className="btn small" onClick={() => adjustSubtitleOffset(500)}>
            {t('player.subtitleLater')}
          </button>
          {subtitleOffsetMs !== 0 && (
            <button className="btn small" onClick={resetSubtitleOffset}>
              {t('player.subtitleReset')}
            </button>
          )}
        </div>
      )}

      <div className="player-tools skip-bar">
        <span className="muted">{t('player.skipLabel')}</span>
        <button className="btn small ghost" onClick={() => markSkip('intro')}>
          {skipDraft?.kind === 'intro' ? t('player.skipIntroEnd') : t('player.skipIntroStart')}
        </button>
        {skip.intro && (
          <>
            <span className="muted small">
              {t('player.skipIntroRange', { from: fmtDuration(skip.intro.start_sec), to: fmtDuration(skip.intro.end_sec) })}
            </span>
            <button className="btn small ghost" onClick={() => skipTo('intro')}>{t('player.skipIntro')}</button>
            <button className="btn small ghost" onClick={() => clearSkip('intro')}>{t('player.skipClear')}</button>
          </>
        )}
        <button className="btn small ghost" onClick={() => markSkip('credits')}>
          {skipDraft?.kind === 'credits' ? t('player.skipCreditsEnd') : t('player.skipCreditsStart')}
        </button>
        {skip.credits && (
          <>
            <span className="muted small">
              {t('player.skipCreditsRange', { from: fmtDuration(skip.credits.start_sec), to: fmtDuration(skip.credits.end_sec) })}
            </span>
            <button className="btn small ghost" onClick={() => skipTo('credits')}>{t('player.skipCredits')}</button>
            <button className="btn small ghost" onClick={() => clearSkip('credits')}>{t('player.skipClear')}</button>
          </>
        )}
      </div>

      {((useTranscode && (levels.length > 1 || subtitleTracks.length > 0 || audioTracks.length > 1)) ||
        tracks.filter((tr) => tr.format === 'ass' || tr.format === 'ssa').length > 0) && (
        <div className="player-tools">
          {useTranscode && levels.length > 1 && (
            <label className="player-tool">
              {t('player.quality')}
              <select value={quality} onChange={(e) => changeQuality(e.target.value)}>
                <option value="auto">{t('player.qualityAuto')}</option>
                {levels.map((l) => (
                  <option key={l.i} value={l.i}>
                    {l.label}
                  </option>
                ))}
              </select>
            </label>
          )}
          {subtitleTracks.length > 0 && (
            <label className="player-tool">
              {t('player.subtitles')}
              <select
                value={subtitleIdx}
                onChange={(e) => changeSubtitle(Number(e.target.value))}
              >
                <option value={-1}>{t('player.subtitlesOff')}</option>
                {subtitleTracks.map((tr) => (
                  <option key={tr.id} value={tr.i}>
                    {tr.label}
                  </option>
                ))}
              </select>
            </label>
          )}
          {audioTracks.length > 1 && (
            <label className="player-tool">
              {t('player.audioTrack')}
              <select value={audioIdx} onChange={(e) => changeAudio(Number(e.target.value))}>
                {audioTracks.map((tr) => (
                  <option key={tr.id} value={tr.i}>
                    {tr.label}
                  </option>
                ))}
              </select>
            </label>
          )}
          {tracks.filter((tr) => tr.format === 'ass' || tr.format === 'ssa').length > 0 && (
            <label className="player-tool">
              {t('player.assSubtitles')}
              <select value={assIdx} onChange={(e) => setAssTrack(e.target.value)}>
                <option value="">{t('player.subtitlesOff')}</option>
                {tracks
                  .filter((tr) => tr.format === 'ass' || tr.format === 'ssa')
                  .map((tr) => (
                    <option key={tr.id} value={tr.id}>
                      {tr.title || tr.lang || t('player.assSubtitles')}
                    </option>
                  ))}
              </select>
            </label>
          )}
        </div>
      )}

      {transcoding && (
        <div className="banner info">{t('player.transcoding')}</div>
      )}

      {hlsErr && (
        <div className="banner warn">
          {hlsErr}{' '}
          {!useTranscode && (
            <button className="btn small" onClick={() => navigate(`/player/${activeId}?transcode=1`)}>
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
              className={`queue-item ${v.id === activeId ? 'current' : ''}`}
              onClick={() => v.id !== activeId && switchEpisode(v.id)}
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
