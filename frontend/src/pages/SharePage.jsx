import { useEffect, useRef, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { publicUrl } from '../api.js';
import { fmtBytes, fmtDuration } from '../i18n';

const BROWSER_PLAYABLE = ['.mp4', '.m4v', '.webm', '.mov', '.ogv'];

export default function SharePage() {
  const { token } = useParams();
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [err, setErr] = useState('');
  const [queue, setQueue] = useState([]);
  const [queueTitle, setQueueTitle] = useState('');
  const [activeIdx, setActiveIdx] = useState(0);
  const videoRef = useRef(null);
  const hlsRef = useRef(null);
  const [useTranscode, setUseTranscode] = useState(false);
  const [transcoding, setTranscoding] = useState(false);
  const [levels, setLevels] = useState([]);
  const [quality, setQuality] = useState('auto');
  const [nativeTracks, setNativeTracks] = useState([]);
  const [subtitleTracks, setSubtitleTracks] = useState([]);
  const [subtitleIdx, setSubtitleIdx] = useState(-1);
  const [pw, setPw] = useState('');
  const [pwInput, setPwInput] = useState('');
  const [askPassword, setAskPassword] = useState(false);

  function shareFetch(path) {
    const headers = pw ? { 'X-Share-Password': pw } : {};
    return fetch(publicUrl(path), { headers });
  }

  useEffect(() => {
    shareFetch(`/share/${token}/info`)
      .then(async (res) => {
        const d = await res.json();
        if (res.status === 403 && d.password_required) {
          setAskPassword(true);
          return;
        }
        if (!res.ok) throw new Error(d.error || t('share.invalid'));
        setAskPassword(false);
        setData(d);
      })
      .catch((e) => setErr(e.message || t('share.invalid')));
  }, [token, pw, t]); // eslint-disable-line react-hooks/exhaustive-deps

  // build the play queue from the share scope
  useEffect(() => {
    if (!data) return;
    if (data.scope === 'series') {
      setQueue(data.items || []);
      setQueueTitle(data.series?.name || '');
    } else if (data.scope === 'playlist') {
      setQueue(data.items || []);
      setQueueTitle(data.playlist?.name || '');
    } else {
      setQueue(data.video ? [data.video] : []);
      setQueueTitle('');
    }
    setActiveIdx(0);
  }, [data]);

  const active = queue[activeIdx];
  const media = (path) => {
    const u = publicUrl(`/share/${token}/video/${active.id}${path}`);
    return pw ? `${u}${u.includes('?') ? '&' : '?'}pw=${encodeURIComponent(pw)}` : u;
  };

  useEffect(() => {
    if (!active || !videoRef.current) return;
    setLevels([]);
    setSubtitleTracks([]);
    setNativeTracks([]);
    setErr('');

    const ext = active.filename?.match(/\.[^.]+$/)?.[0]?.toLowerCase() || '';
    const shouldTranscode = !BROWSER_PLAYABLE.includes(ext);
    setUseTranscode(shouldTranscode);
    if (shouldTranscode) {
      let cancelled = false;
      import('hls.js')
        .then(({ default: Hls }) => {
          if (cancelled) return;
          const hls = new Hls({
            startPosition: 0,
            maxBufferLength: 30,
            maxMaxBufferLength: 90,
            xhrSetup: (xhr) => {
              if (pw) xhr.setRequestHeader('X-Share-Password', pw);
            },
          });
          hlsRef.current = hls;
          setTranscoding(true);
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
          hls.on(Hls.Events.LEVEL_SWITCHED, (_event, d) => setQuality(String(d.level)));
          hls.on(Hls.Events.SUBTITLE_TRACKS_UPDATED, (_event, d) => {
            const list = d.subtitleTracks || [];
            setSubtitleTracks(
              list.map((tr, i) => ({
                i,
                id: tr.id,
                label: tr.name || tr.lang || `${t('player.subtitles')} ${i + 1}`,
              })),
            );
            setSubtitleIdx(hls.subtitleTrack);
          });
          hls.on(Hls.Events.SUBTITLE_TRACK_SWITCH, (_event, d) => {
            setSubtitleIdx(d.id ?? -1);
          });
          hls.on(Hls.Events.ERROR, (_event, d) => {
            if (!d.fatal) return;
            if (d.type === Hls.ErrorTypes.NETWORK_ERROR) {
              hls.startLoad();
              return;
            }
            setErr(t('player.transcodeFailed', { detail: d.details || 'unknown' }));
            setTranscoding(false);
          });
          hls.attachMedia(videoRef.current);
          hls.loadSource(media('/hls/playlist.m3u8'));
        })
        .catch(() => {
          if (!cancelled) setErr(t('share.invalid'));
        });
      return () => {
        cancelled = true;
        if (hlsRef.current) {
          hlsRef.current.destroy();
          hlsRef.current = null;
        }
      };
    }

    // native playback: load subtitle tracks so the CC menu works
    shareFetch(`/share/${token}/video/${active.id}/subtitle-tracks`)
      .then((res) => res.json())
      .then((d) => setNativeTracks(d.items || []))
      .catch(() => setNativeTracks([]));
    return undefined;
  }, [active, token, pw, t]); // eslint-disable-line react-hooks/exhaustive-deps

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

  function playNext() {
    if (activeIdx < queue.length - 1) setActiveIdx(activeIdx + 1);
  }

  if (err) {
    return (
      <div className="container share-page">
        <div className="form-error">{err}</div>
        <Link className="btn ghost" to="/">
          {t('share.backToSite')}
        </Link>
      </div>
    );
  }
  if (askPassword && !data) {
    return (
      <div className="container share-page">
        <form
          className="modal share-password"
          onSubmit={(e) => {
            e.preventDefault();
            setErr('');
            setPw(pwInput);
          }}
        >
          <h3>{t('share.passwordTitle')}</h3>
          <input
            type="password"
            value={pwInput}
            onChange={(e) => setPwInput(e.target.value)}
            placeholder={t('share.passwordPlaceholder')}
            autoFocus
          />
          <div className="modal-actions">
            <button className="btn primary" type="submit">
              {t('common.save')}
            </button>
            <Link className="btn ghost" to="/">
              {t('share.backToSite')}
            </Link>
          </div>
        </form>
      </div>
    );
  }
  if (!data || !active) {
    return (
      <div className="container share-page">
        <div className="loading">{t('common.loading')}</div>
      </div>
    );
  }

  const streamUrl = useTranscode ? undefined : media('/stream');

  return (
    <div className="container share-page">
      <div className="share-head">
        <Link className="btn ghost" to="/">
          {t('share.backToSite')}
        </Link>
        <div>
          <h1>{queueTitle || active.title}</h1>
          <div className="detail-facts">
            {active.year > 0 && <span>{active.year}</span>}
            <span>{fmtDuration(active.duration_sec)}</span>
            {active.width > 0 && (
              <span>
                {active.width}×{active.height}
              </span>
            )}
            <span>{active.video_codec}</span>
            {active.size_bytes > 0 && <span>{fmtBytes(active.size_bytes)}</span>}
            {queue.length > 1 && <span>{activeIdx + 1}/{queue.length}</span>}
          </div>
          {active.synopsis && <p className="synopsis">{active.synopsis}</p>}
        </div>
      </div>

      <video
        ref={videoRef}
        className="player"
        controls
        autoPlay
        src={streamUrl}
        poster={active.has_poster ? media('/poster') : undefined}
        onEnded={playNext}
      >
        {!useTranscode &&
          nativeTracks.map((tr) => (
            <track
              key={tr.id}
              kind="subtitles"
              srcLang={tr.lang || undefined}
              label={tr.title || tr.lang || t('player.subtitles')}
              src={media(`/subtitles/${tr.id}`)}
              default={tr.is_active}
            />
          ))}
      </video>

      {useTranscode && (levels.length > 1 || subtitleTracks.length > 0) && (
        <div className="player-tools">
          {levels.length > 1 && (
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
              <select value={subtitleIdx} onChange={(e) => changeSubtitle(Number(e.target.value))}>
                <option value={-1}>{t('player.subtitlesOff')}</option>
                {subtitleTracks.map((tr) => (
                  <option key={tr.id} value={tr.i}>
                    {tr.label}
                  </option>
                ))}
              </select>
            </label>
          )}
          <a className="btn small" href={media('/download')}>
            {t('common.download')}
          </a>
        </div>
      )}

      {transcoding && <div className="banner info">{t('player.transcoding')}</div>}

      {queue.length > 1 && (
        <div className="queue">
          <h3>{t('player.queue', { count: queue.length })}</h3>
          {queue.map((v, i) => (
            <button
              key={v.id}
              className={`queue-item ${i === activeIdx ? 'current' : ''}`}
              onClick={() => i !== activeIdx && setActiveIdx(i)}
            >
              <span className="queue-idx">
                {v.season > 0
                  ? `S${String(v.season).padStart(2, '0')}E${String(v.episode).padStart(2, '0')}`
                  : i + 1}
              </span>
              <span className="queue-title">{v.title}</span>
              {i === activeIdx && <span className="queue-now">{t('player.nowPlaying')}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
