import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

export default function DownloadDialog({ video, onClose }) {
  const { t } = useTranslation();
  const [data, setData] = useState(null);
  const [err, setErr] = useState('');
  const [container, setContainer] = useState('mkv');
  const [audioIdx, setAudioIdx] = useState(null);
  const [subs, setSubs] = useState(new Set());
  const [sidecars, setSidecars] = useState(new Set());

  useEffect(() => {
    api(`/videos/${video.id}/tracks`)
      .then((d) => {
        setData(d);
        setContainer(d.container === 'mp4' ? 'mp4' : 'mkv');
        setAudioIdx(d.audio?.[0]?.index ?? null);
      })
      .catch((e) => setErr(e.message));
  }, [video.id]);

  const url = useMemo(() => {
    if (!data) return null;
    const p = new URLSearchParams({ container });
    if (audioIdx != null) p.set('audio', String(audioIdx));
    if (subs.size > 0) p.set('sub', [...subs].join(','));
    if (sidecars.size > 0) p.set('sidecar', [...sidecars].join(','));
    return mediaUrl(`/videos/${video.id}/download/remux?${p.toString()}`);
  }, [data, container, audioIdx, subs, sidecars, video.id]);

  function toggleSub(idx) {
    setSubs((prev) => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }

  function toggleSidecar(id) {
    setSidecars((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h3>{t('download.title', { title: video.title })}</h3>
        {err && <div className="form-error">{err}</div>}
        {!data && !err && <div className="loading">{t('common.loading')}</div>}
        {data && (
          <>
            <div className="field-row">
              <span>{t('download.container')}</span>
              <label>
                <input type="radio" name="container" checked={container === 'mkv'} onChange={() => setContainer('mkv')} />
                MKV
              </label>
              <label>
                <input type="radio" name="container" checked={container === 'mp4'} onChange={() => setContainer('mp4')} />
                MP4
              </label>
            </div>

            <h4>{t('download.audio')}</h4>
            {data.audio.length === 0 ? (
              <p className="muted">{t('download.noAudio')}</p>
            ) : (
              data.audio.map((a) => (
                <label key={a.index} className="track-option">
                  <input
                    type="radio"
                    name="audio"
                    checked={audioIdx === a.index}
                    onChange={() => setAudioIdx(a.index)}
                  />
                  {a.title || a.language || t('download.audioTrack')} ({a.codec})
                </label>
              ))
            )}

            <h4>{t('download.subtitles')}</h4>
            {data.subtitle.length === 0 && data.sidecar.length === 0 ? (
              <p className="muted">{t('download.noSubtitles')}</p>
            ) : (
              <>
                {data.subtitle.map((s) => (
                  <label key={s.index} className="track-option">
                    <input type="checkbox" checked={subs.has(s.index)} onChange={() => toggleSub(s.index)} />
                    {s.title || s.language || t('download.embeddedSub')} ({s.codec})
                  </label>
                ))}
                {data.sidecar.map((s) => (
                  <label key={s.id} className="track-option">
                    <input type="checkbox" checked={sidecars.has(s.id)} onChange={() => toggleSidecar(s.id)} />
                    {s.title || s.lang || t('download.sidecarSub')} ({s.kind})
                  </label>
                ))}
              </>
            )}

            <div className="modal-actions">
              <button className="btn ghost" onClick={onClose}>
                {t('common.cancel')}
              </button>
              <a className="btn ghost" href={mediaUrl(`/videos/${video.id}/download`)}>
                {t('download.original')}
              </a>
              <a className="btn primary" href={url} onClick={onClose}>
                {t('download.download')}
              </a>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
