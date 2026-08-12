import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';
import Poster from '../components/Poster.jsx';
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
  const [shareHours, setShareHours] = useState(168);
  const [shares, setShares] = useState([]);
  const [createdShare, setCreatedShare] = useState('');
  const [copied, setCopied] = useState(false);
  const [shareErr, setShareErr] = useState('');
  const [subtitleBusy, setSubtitleBusy] = useState(false);

  useEffect(() => {
    api(`/videos/${id}`).then(setVideo).catch((e) => setErr(e.message));
    api('/playlists').then((d) => setPlaylists(d.items)).catch(() => {});
    api(`/videos/${id}/shares`).then((d) => setShares(d.items)).catch(() => {});
  }, [id]);

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
      await api(`/videos/${video.id}/scrape`, { method: 'POST' });
      const fresh = await api(`/videos/${video.id}`);
      setVideo(fresh);
      setMsg(t('video.scrapeDone'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setScraping(false);
    }
  }

  async function createShareLink(e) {
    e.preventDefault();
    setShareErr('');
    try {
      const d = await api(`/videos/${video.id}/share`, {
        method: 'POST',
        body: { hours: Number(shareHours) || 168 },
      });
      setCreatedShare(`${window.location.origin}${d.url}`);
      const list = await api(`/videos/${video.id}/shares`);
      setShares(list.items);
    } catch (e2) {
      setShareErr(e2.message);
    }
  }

  async function revokeShare(token) {
    setShareErr('');
    try {
      await api(`/share/${token}`, { method: 'DELETE' });
      setShares((prev) => prev.filter((s) => s.token !== token));
      if (createdShare && createdShare.endsWith(token)) setCreatedShare('');
    } catch (e2) {
      setShareErr(e2.message);
    }
  }

  async function copyShare() {
    try {
      await navigator.clipboard.writeText(createdShare);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard unavailable; the link is still visible to copy manually
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
      setMsg(t('video.subtitleRemoved'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setSubtitleBusy(false);
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
            <a className="btn" href={mediaUrl(`/videos/${video.id}/download`)}>
              {t('common.download')}
            </a>
            <button className="btn" onClick={() => setShowShare(true)}>
              {t('video.share')}
            </button>
            {user?.role === 'admin' && (
              <>
                <button className="btn ghost" onClick={scrape} disabled={scraping}>
                  {scraping ? t('video.scraping') : t('video.scrape')}
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

      {showShare && (
        <div className="modal-backdrop" onClick={() => setShowShare(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t('video.shareTitle')}</h3>
            <form onSubmit={createShareLink} className="inline-form">
              <input
                type="number"
                min="1"
                max="8760"
                value={shareHours}
                onChange={(e) => setShareHours(e.target.value)}
                placeholder={t('video.shareHours')}
              />
              <button className="btn small primary">{t('video.shareCreate')}</button>
            </form>
            {createdShare && (
              <div className="share-link-row">
                <code className="share-link">{createdShare}</code>
                <button className="btn small" onClick={copyShare}>
                  {copied ? t('video.shareCopied') : t('video.shareCopy')}
                </button>
              </div>
            )}
            {shareErr && <div className="form-error">{shareErr}</div>}
            {shares.length > 0 && (
              <div className="share-list">
                <h4>{t('video.shareList')}</h4>
                {shares.map((s) => (
                  <div key={s.token} className="share-row">
                    <span className="muted">
                      {t('video.shareExpires', {
                        date: new Date(s.expires_at).toLocaleString(),
                      })}
                    </span>
                    <button className="btn small ghost" onClick={() => revokeShare(s.token)}>
                      {t('video.shareRevoke')}
                    </button>
                  </div>
                ))}
              </div>
            )}
            <button className="btn ghost" onClick={() => setShowShare(false)}>
              {t('common.close')}
            </button>
          </div>
        </div>
      )}

      <div className="back-link">
        <Link to="/">← {t('nav.home')}</Link>
      </div>
    </div>
  );
}
