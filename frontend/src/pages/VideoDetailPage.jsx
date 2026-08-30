import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import Poster from '../components/Poster.jsx';
import DownloadDialog from '../components/DownloadDialog.jsx';
import ShareModal from '../components/ShareModal.jsx';
import SubtitleSearchModal from '../components/SubtitleSearchModal.jsx';
import { useAuth } from '../auth.jsx';
import { fmtBytes, fmtDuration } from '../i18n';

export default function VideoDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const { t } = useTranslation();
  const [video, setVideo] = useState(null);
  const [playlists, setPlaylists] = useState([]);
  const [showPlaylistPicker, setShowPlaylistPicker] = useState(false);
  const [newPlaylistName, setNewPlaylistName] = useState('');
  const [editing, setEditing] = useState(false);
  const [scraping, setScraping] = useState(false);
  const [form, setForm] = useState(null);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [showShare, setShowShare] = useState(false);
  const [showDownload, setShowDownload] = useState(false);
  const [showSubSearch, setShowSubSearch] = useState(false);
  const [transcript, setTranscript] = useState(null);
  const [transcribing, setTranscribing] = useState(false);
  const [scrapeProvider, setScrapeProvider] = useState('tmdb');
  const [scrapeForce, setScrapeForce] = useState(false);
  const [tags, setTags] = useState([]);
  const [tagInput, setTagInput] = useState('');
  const [tagBusy, setTagBusy] = useState(false);
  const [subtitleBusy, setSubtitleBusy] = useState(false);
  const [tracks, setTracks] = useState([]);
  const [setGlobal, setSetGlobal] = useState(false);

  useEffect(() => {
    api(`/videos/${id}`).then(setVideo).catch((e) => setErr(e.message));
    api('/playlists').then((d) => setPlaylists(d.items)).catch(() => {});
    api(`/videos/${id}/subtitle-tracks`).then((d) => setTracks(d.items)).catch(() => setTracks([]));
    api(`/videos/${id}/transcripts`).then(setTranscript).catch(() => setTranscript(null));
    api(`/videos/${id}/tags`).then((d) => setTags(d.items || [])).catch(() => setTags([]));
  }, [id]);

  async function addTag(e) {
    e.preventDefault();
    const name = tagInput.trim().toLowerCase();
    if (!name) return;
    try {
      const d = await api(`/videos/${id}/tags`, { method: 'POST', body: { name } });
      setTags((prev) => (prev.some((t) => t.id === d.id) ? prev : [...prev, d]));
      setTagInput('');
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function removeTag(tagId) {
    try {
      await api(`/videos/${id}/tags/${tagId}`, { method: 'DELETE' });
      setTags((prev) => prev.filter((t) => t.id !== tagId));
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function analyze() {
    setTagBusy(true);
    setErr('');
    try {
      await api(`/videos/${id}/analyze`, { method: 'POST' });
      const d = await api(`/videos/${id}/tags`);
      setTags(d.items || []);
      setMsg(t('video.analyzeDone'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setTagBusy(false);
    }
  }

  async function transcribe() {
    setTranscribing(true);
    setErr('');
    try {
      const d = await api(`/videos/${id}/transcribe`, { method: 'POST' });
      setTranscript(d);
      setMsg(t('video.transcriptDone'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setTranscribing(false);
    }
  }

  async function toggleFavorite() {
    try {
      if (video.is_favorite) {
        await api(`/users/me/favorites/${video.id}`, { method: 'DELETE' });
      } else {
        await api('/users/me/favorites', { method: 'POST', body: { video_id: video.id } });
      }
      setVideo({ ...video, is_favorite: !video.is_favorite });
    } catch (e) {
      setErr(e.message);
    }
  }

  async function addToPlaylist(playlistId) {
    try {
      await api(`/playlists/${playlistId}/items`, {
        method: 'POST',
        body: { video_id: video.id },
      });
      setMsg(t('video.addedToPlaylist'));
      setShowPlaylistPicker(false);
    } catch (e) {
      setErr(e.message);
    }
  }

  async function createAndAdd(e) {
    e.preventDefault();
    try {
      const p = await api('/playlists', {
        method: 'POST',
        body: { name: newPlaylistName },
      });
      await addToPlaylist(p.id);
      setPlaylists((prev) => [...prev, p]);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function saveEdit(e) {
    e.preventDefault();
    try {
      await api(`/videos/${video.id}`, {
        method: 'PATCH',
        body: {
          title: form.title,
          synopsis: form.synopsis,
          year: Number(form.year) || 0,
          genres: form.genres
            .split(',')
            .map((g) => g.trim())
            .filter(Boolean),
        },
      });
      setEditing(false);
      setVideo({
        ...video,
        title: form.title,
        synopsis: form.synopsis,
        year: Number(form.year) || 0,
        genres: form.genres
          .split(',')
          .map((g) => g.trim())
          .filter(Boolean),
      });
      setMsg(t('video.metaUpdated'));
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function scrape() {
    setScraping(true);
    setErr('');
    try {
      await api(
        `/videos/${video.id}/scrape?provider=${scrapeProvider}&force=${scrapeForce ? 1 : 0}`,
        { method: 'POST' },
      );
      const fresh = await api(`/videos/${video.id}`);
      setVideo(fresh);
      setMsg(t('video.scrapeDone'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setScraping(false);
    }
  }

  async function uploadSubtitle(e) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setSubtitleBusy(true);
    setErr('');
    try {
      const fd = new FormData();
      fd.append('subtitle', file);
      await api(`/videos/${video.id}/subtitles`, { method: 'POST', form: fd });
      const fresh = await api(`/videos/${video.id}`);
      setVideo(fresh);
      const td = await api(`/videos/${video.id}/subtitle-tracks`);
      setTracks(td.items);
      setMsg(t('video.subtitleUploaded'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setSubtitleBusy(false);
    }
  }

  async function extractSubtitle() {
    setSubtitleBusy(true);
    setErr('');
    try {
      await api(`/videos/${video.id}/subtitles/extract`, { method: 'POST' });
      const fresh = await api(`/videos/${video.id}`);
      setVideo(fresh);
      const td = await api(`/videos/${video.id}/subtitle-tracks`);
      setTracks(td.items);
      setMsg(t('video.subtitleExtracted'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setSubtitleBusy(false);
    }
  }

  async function removeSubtitle() {
    setSubtitleBusy(true);
    setErr('');
    try {
      await api(`/videos/${video.id}/subtitles`, { method: 'DELETE' });
      const fresh = await api(`/videos/${video.id}`);
      setVideo(fresh);
      const td = await api(`/videos/${video.id}/subtitle-tracks`);
      setTracks(td.items);
      setMsg(t('video.subtitleRemoved'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setSubtitleBusy(false);
    }
  }

  async function changeSubtitlePref(trackId) {
    setErr('');
    try {
      if (!trackId) {
        await api(`/videos/${video.id}/subtitles/preference`, { method: 'DELETE' });
      } else {
        await api(`/videos/${video.id}/subtitles/${trackId}/active`, { method: 'PUT' });
        if (user?.role === 'admin' && setGlobal) {
          await api(`/videos/${video.id}/subtitles/${trackId}/default`, { method: 'PUT' });
        }
      }
      const d = await api(`/videos/${video.id}/subtitle-tracks`);
      setTracks(d.items);
      setMsg(t('video.subtitlePrefSaved'));
    } catch (e2) {
      setErr(e2.message);
    }
  }

  if (err && !video) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!video) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  return (
    <div className="container">
      <div className="detail">
        <Poster video={video} className="detail-poster" />
        <div className="detail-info">
          <div className="detail-meta-top">
            <span className="library-tag">{video.library_name}</span>
            {video.year > 0 && <span className="year-tag">{video.year}</span>}
            {video.is_favorite && <span className="fav-star">{t('video.favorited')}</span>}
          </div>
          <h1>{video.title}</h1>
          <div className="detail-facts">
            <span>{fmtDuration(video.duration_sec)}</span>
            {video.width > 0 && <span>{video.width}×{video.height}</span>}
            <span>{video.video_codec}</span>
            <span>{fmtBytes(video.size_bytes)}</span>
            {video.genres?.length > 0 && <span>{video.genres.join(' / ')}</span>}
          </div>
          {video.synopsis ? (
            <p className="synopsis">{video.synopsis}</p>
          ) : (
            <p className="synopsis muted">{t('video.noSynopsis')}</p>
          )}
          {(tags.length > 0 || user?.role === 'admin') && (
            <div className="tag-box">
              {tags.map((tg) => (
                <span key={tg.id} className="tag-chip">
                  {tg.name}
                  {tg.kind === 'auto' && <span className="tag-auto" title={t('video.tagAuto')}>✦</span>}
                  <button className="tag-remove" onClick={() => removeTag(tg.id)} aria-label={t('common.remove')}>×</button>
                </span>
              ))}
              <form className="tag-add" onSubmit={addTag}>
                <input
                  placeholder={t('video.tagPlaceholder')}
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  maxLength={50}
                />
              </form>
              {user?.role === 'admin' && (
                <button className="btn small ghost" onClick={analyze} disabled={tagBusy}>
                  {tagBusy ? t('video.analyzing') : t('video.analyze')}
                </button>
              )}
            </div>
          )}
          <div className="detail-actions">
            <button className="btn primary big" onClick={() => navigate(`/player/${video.id}`)}>
              ▶ {video.progress_sec > 5 ? t('video.resume') : t('video.play')}
            </button>
            <button className="btn" onClick={toggleFavorite}>
              {video.is_favorite ? t('video.unfavorite') : t('video.favorite')}
            </button>
            <button className="btn" onClick={() => setShowPlaylistPicker((v) => !v)}>
              {t('video.addToPlaylist')}
            </button>
            <button className="btn" onClick={() => setShowDownload(true)}>
              {t('common.download')}
            </button>
            <button className="btn" onClick={() => setShowShare(true)}>
              {t('video.share')}
            </button>
            {user?.role === 'admin' && (
              <>
                <button className="btn ghost" onClick={scrape} disabled={scraping}>
                  {scraping ? t('video.scraping') : t('video.scrape')}
                </button>
                <select value={scrapeProvider} onChange={(e) => setScrapeProvider(e.target.value)}>
                  <option value="tmdb">{t('video.scrapeProviderTmdb')}</option>
                  <option value="custom">{t('video.scrapeProviderCustom')}</option>
                </select>
                <label className="scrape-force">
                  <input type="checkbox" checked={scrapeForce} onChange={(e) => setScrapeForce(e.target.checked)} />
                  {t('video.scrapeForce')}
                </label>
                <button className="btn ghost" onClick={transcribe} disabled={transcribing}>
                  {transcribing ? t('video.transcribing') : t('video.transcribe')}
                </button>
                <label className="btn ghost" disabled={subtitleBusy}>
                  {t('video.subtitleUpload')}
                  <input
                    type="file"
                    accept=".srt,.vtt,.ass,.ssa"
                    hidden
                    onChange={uploadSubtitle}
                  />
                </label>
                <button
                  className="btn ghost"
                  onClick={extractSubtitle}
                  disabled={subtitleBusy}
                >
                  {subtitleBusy ? t('video.subtitleBusy') : t('video.subtitleExtract')}
                </button>
                <button className="btn ghost" onClick={() => setShowSubSearch(true)}>
                  {t('video.subtitleSearch')}
                </button>
                {video.has_subtitle && (
                  <button className="btn ghost" onClick={removeSubtitle} disabled={subtitleBusy}>
                    {t('video.subtitleRemove')}
                  </button>
                )}
                <button
                  className="btn ghost"
                  onClick={() => {
                    setForm({
                      title: video.title,
                      synopsis: video.synopsis,
                      year: video.year || '',
                      genres: (video.genres || []).join(', '),
                    });
                    setEditing(true);
                  }}
                >
                  {t('video.editMetadata')}
                </button>
              </>
            )}
          </div>
          {msg && <div className="toast toast-success">{msg}</div>}
          {err && <div className="form-error">{err}</div>}
          {tracks.length > 0 && (
            <div className="subtitle-pref">
              <label>
                {t('video.subtitlePreference')}
                <select
                  value={tracks.find((tr) => tr.is_active)?.id || ''}
                  onChange={(e) => changeSubtitlePref(e.target.value)}
                >
                  <option value="">{t('video.subtitleGlobal')}</option>
                  {tracks.map((tr) => (
                    <option key={tr.id} value={tr.id}>
                      {tr.title || tr.lang || t('video.subtitleTrack')}
                    </option>
                  ))}
                </select>
              </label>
              {user?.role === 'admin' && (
                <label className="subtitle-global-check">
                  <input
                    type="checkbox"
                    checked={setGlobal}
                    onChange={(e) => setSetGlobal(e.target.checked)}
                  />
                  {t('video.subtitleSetGlobal')}
                </label>
              )}
            </div>
          )}
          {transcript && transcript.status !== 'none' && (
            <div className="card transcript-box">
              <div className="playlist-name">
                {t('video.transcript')}
                <span
                  className={`status-badge status-${
                    transcript.status === 'done' ? 'completed' : transcript.status === 'failed' ? 'error' : 'queued'
                  }`}
                >
                  {transcript.status === 'done'
                    ? t('video.transcriptReady')
                    : transcript.status === 'failed'
                      ? t('video.transcriptFailed')
                      : t('video.transcribing')}
                </span>
              </div>
              {transcript.error && <div className="form-error small">{transcript.error}</div>}
              {transcript.preview && <p className="muted small">{transcript.preview}</p>}
            </div>
          )}
        </div>
      </div>

      {showPlaylistPicker && (
        <div className="modal-backdrop" onClick={() => setShowPlaylistPicker(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t('video.playlistPickerTitle')}</h3>
            {playlists.map((p) => (
              <div key={p.id} className="playlist-row">
                <span>{p.name}</span>
                <button className="btn small" onClick={() => addToPlaylist(p.id)}>
                  {t('video.add')}
                </button>
              </div>
            ))}
            <form onSubmit={createAndAdd} className="inline-form">
              <input
                value={newPlaylistName}
                onChange={(e) => setNewPlaylistName(e.target.value)}
                placeholder={t('video.newPlaylistName')}
              />
              <button className="btn small primary">{t('video.createAndAdd')}</button>
            </form>
            <button className="btn ghost" onClick={() => setShowPlaylistPicker(false)}>
              {t('common.close')}
            </button>
          </div>
        </div>
      )}

      {editing && (
        <div className="modal-backdrop" onClick={() => setEditing(false)}>
          <form className="modal" onClick={(e) => e.stopPropagation()} onSubmit={saveEdit}>
            <h3>{t('video.editTitle')}</h3>
            <label>
              {t('video.fieldTitle')}
              <input
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
              />
            </label>
            <label>
              {t('video.fieldYear')}
              <input
                type="number"
                value={form.year}
                onChange={(e) => setForm({ ...form, year: e.target.value })}
              />
            </label>
            <label>
              {t('video.fieldGenres')}
              <input
                value={form.genres}
                onChange={(e) => setForm({ ...form, genres: e.target.value })}
              />
            </label>
            <label>
              {t('video.fieldSynopsis')}
              <textarea
                rows={4}
                value={form.synopsis}
                onChange={(e) => setForm({ ...form, synopsis: e.target.value })}
              />
            </label>
            <div className="modal-actions">
              <button type="submit" className="btn primary">{t('common.save')}</button>
              <button type="button" className="btn ghost" onClick={() => setEditing(false)}>
                {t('common.cancel')}
              </button>
            </div>
          </form>
        </div>
      )}

      {showShare && <ShareModal kind="videos" id={video.id} onClose={() => setShowShare(false)} />}
      {showDownload && <DownloadDialog video={video} onClose={() => setShowDownload(false)} />}
      {showSubSearch && (
        <SubtitleSearchModal
          video={video}
          onClose={() => setShowSubSearch(false)}
          onDownloaded={() => {
            setMsg(t('video.subtitleDownloaded'));
            api(`/videos/${video.id}/subtitle-tracks`)
              .then((d) => setTracks(d.items))
              .catch(() => {});
          }}
        />
      )}

      <div className="back-link">
        <Link to="/">← {t('nav.home')}</Link>
      </div>
    </div>
  );
}
