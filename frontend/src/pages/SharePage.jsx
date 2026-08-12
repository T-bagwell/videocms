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
  const videoRef = useRef(null);
  const hlsRef = useRef(null);
  const [useTranscode, setUseTranscode] = useState(false);
  const [transcoding, setTranscoding] = useState(false);
  const [levels, setLevels] = useState([]);
  const [quality, setQuality] = useState('auto');
  const [subtitleAvailable, setSubtitleAvailable] = useState(false);
  const [subtitlesOn, setSubtitlesOn] = useState(true);

  useEffect(() => {
    fetch(publicUrl(`/share/${token}/info`))
      .then((res) =>
        res.json().then((d) => ({ ok: res.ok, data: d })),
      )
      .then(({ ok, data: d }) => {
        if (!ok) throw new Error(d.error || t('share.invalid'));
        setData(d);
      })
      .catch((e) => setErr(e.message || t('share.invalid')));
  }, [token, t]);

  useEffect(() => {
    if (!data || !videoRef.current) return;
    const video = data.video;
    const ext = video.filename?.match(/\.[^.]+$/)?.[0]?.toLowerCase() || '';
    const shouldTranscode = !BROWSER_PLAYABLE.includes(ext);
    setUseTranscode(shouldTranscode);
    if (!shouldTranscode) return;

    let cancelled = false;
    import('hls.js')
      .then(({ default: Hls }) => {
        if (cancelled) return;
        const hls = new Hls({
          startPosition: 0,
          maxBufferLength: 30,
          maxMaxBufferLength: 90,
        });
        hlsRef.current = hls;
        setTranscoding(true);
        setLevels([]);
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
          const has = (d.subtitleTracks || []).length > 0;
          setSubtitleAvailable(has);
          if (has) hls.subtitleTrack = 0;
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
        hls.loadSource(publicUrl(`/share/${token}/hls/playlist.m3u8`));
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
  }, [data, token, t]);

  function changeQuality(q) {
    setQuality(q);
    const hls = hlsRef.current;
    if (!hls || !hls.levels || !hls.levels.length) return;
    hls.currentLevel = q === 'auto' ? -1 : Number(q);
  }

  function toggleSubtitles() {
    const hls = hlsRef.current;
    if (!hls) return;
    hls.subtitleTrack = subtitlesOn ? -1 : 0;
    setSubtitlesOn(!subtitlesOn);
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
  if (!data) {
    return (
      <div className="container share-page">
        <div className="loading">{t('common.loading')}</div>
      </div>
    );
  }

  const video = data.video;
  const streamUrl = useTranscode ? undefined : publicUrl(`/share/${token}/stream`);

  return (
    <div className="container share-page">
      <div className="share-head">
        <Link className="btn ghost" to="/">
          {t('share.backToSite')}
        </Link>
        <div>
          <h1>{video.title}</h1>
          <div className="detail-facts">
            {video.year > 0 && <span>{video.year}</span>}
            <span>{fmtDuration(video.duration_sec)}</span>
            {video.width > 0 && (
              <span>
                {video.width}×{video.height}
              </span>
            )}
            <span>{video.video_codec}</span>
            {video.size_bytes > 0 && <span>{fmtBytes(video.size_bytes)}</span>}
          </div>
          {video.synopsis && <p className="synopsis">{video.synopsis}</p>}
        </div>
      </div>

      <video
        ref={videoRef}
        className="player"
        controls
        autoPlay
        src={streamUrl}
        poster={video.has_poster ? publicUrl(`/share/${token}/poster`) : undefined}
      >
        {video.has_subtitle && !useTranscode && (
          <track
            kind="subtitles"
            srcLang="und"
            label={t('player.subtitles')}
            src={publicUrl(`/share/${token}/subtitles`)}
            default
          />
        )}
      </video>

      {(useTranscode || video.has_subtitle) && (
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
          {useTranscode && subtitleAvailable && (
            <button className="btn small" onClick={toggleSubtitles}>
              {subtitlesOn ? t('player.subtitlesOff') : t('player.subtitlesOn')}
            </button>
          )}
          <a className="btn small" href={publicUrl(`/share/${token}/download`)}>
            {t('common.download')}
          </a>
        </div>
      )}

      {transcoding && <div className="banner info">{t('player.transcoding')}</div>}
    </div>
  );
}
