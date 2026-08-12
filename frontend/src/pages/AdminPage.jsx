import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';
import { useAuth } from '../auth.jsx';
import PathPicker from '../components/PathPicker.jsx';
import { fmtBytes } from '../i18n';
import BlockedAdmin from './BlockedAdmin.jsx';

export default function AdminPage() {
  const [tab, setTab] = useState('overview');
  const { t } = useTranslation();

  return (
    <div className="container">
      <h1>{t('admin.title')}</h1>
      <div className="tabs">
        <button className={tab === 'overview' ? 'tab active' : 'tab'} onClick={() => setTab('overview')}>
          {t('admin.tabOverview')}
        </button>
        <button className={tab === 'libraries' ? 'tab active' : 'tab'} onClick={() => setTab('libraries')}>
          {t('admin.tabLibraries')}
        </button>
        <button className={tab === 'videos' ? 'tab active' : 'tab'} onClick={() => setTab('videos')}>
          {t('admin.tabVideos')}
        </button>
        <button className={tab === 'blocked' ? 'tab active' : 'tab'} onClick={() => setTab('blocked')}>
          {t('admin.tabBlocked')}
        </button>
        <button className={tab === 'users' ? 'tab active' : 'tab'} onClick={() => setTab('users')}>
          {t('admin.tabUsers')}
        </button>
      </div>
      {tab === 'overview' && <Overview />}
      {tab === 'libraries' && <Libraries />}
      {tab === 'videos' && <VideoAdmin />}
      {tab === 'blocked' && <BlockedAdmin />}
      {tab === 'users' && <Users />}
    </div>
  );
}

function Overview() {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);
  useEffect(() => {
    api('/admin/stats').then(setStats).catch(() => {});
  }, []);
  if (!stats) return <div className="loading">{t('common.loading')}</div>;
  return (
    <>
      <div className="stats-grid">
        <div className="card stat"><div className="stat-num">{stats.videos}</div><div>{t('admin.statsVideos')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.libraries}</div><div>{t('admin.statsLibraries')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.users}</div><div>{t('admin.statsUsers')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.playlists}</div><div>{t('admin.statsPlaylists')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.favorites}</div><div>{t('admin.statsFavorites')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.series}</div><div>{t('admin.statsSeries')}</div></div>
        <div className="card stat"><div className="stat-num">{fmtBytes(stats.total_bytes)}</div><div>{t('admin.statsStorage')}</div></div>
        {stats.videos_missing > 0 && (
          <div className="card stat warn"><div className="stat-num">{stats.videos_missing}</div><div>{t('admin.statsMissing')}</div></div>
        )}
      </div>
      <a className="btn ghost" href={mediaUrl('/admin/export')}>
        {t('admin.export')}
      </a>
    </>
  );
}

function Libraries() {
  const { t, i18n } = useTranslation();
  const [libs, setLibs] = useState([]);
  const [name, setName] = useState('');
  const [path, setPath] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  function refresh() {
    api('/libraries').then((d) => setLibs(d.items)).catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  // auto-refresh while any library is scanning so progress keeps updating
  useEffect(() => {
    const hasScanning = libs.some((l) => l.scan_status === 'scanning');
    if (!hasScanning) return;
    const t = setInterval(refresh, 3000);
    return () => clearInterval(t);
  }, [libs]);

  async function add(e) {
    e.preventDefault();
    try {
      await api('/libraries', { method: 'POST', body: { name, path } });
      setName('');
      setPath('');
      setMsg(t('admin.libAdded'));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function scan(id) {
    try {
      await api(`/libraries/${id}/scan`, { method: 'POST' });
      setMsg(t('admin.scanStarted'));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function cancelScan(id) {
    try {
      await api(`/libraries/${id}/scan/cancel`, { method: 'POST' });
      setMsg(t('admin.stopRequested'));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function openFolder(l) {
    setErr('');
    setMsg('');
    try {
      const d = await api(`/libraries/${l.id}/open`, { method: 'POST' });
      setMsg(t('admin.folderOpened', { path: d.path }));
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function toggleLibraryBlock(l) {
    if (l.blocked) {
      if (!window.confirm(t('admin.unblockLibraryConfirm', { name: l.name }))) return;
    } else if (!window.confirm(t('admin.blockLibraryConfirm', { name: l.name }))) {
      return;
    }
    setErr('');
    setMsg('');
    try {
      await api(`/libraries/${l.id}`, { method: 'PATCH', body: { blocked: !l.blocked } });
      setMsg(l.blocked
        ? t('admin.libUnblocked', { name: l.name })
        : t('admin.libBlocked', { name: l.name }));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function remove(id) {
    if (!window.confirm(t('admin.deleteLibraryConfirm'))) return;
    try {
      await api(`/libraries/${id}`, { method: 'DELETE' });
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card inline-form" onSubmit={add}>
        <input placeholder={t('admin.libName')} value={name} onChange={(e) => setName(e.target.value)} required />
        <div className="path-field">
          <input
            placeholder={t('admin.libPath')}
            value={path}
            onChange={(e) => setPath(e.target.value)}
            required
          />
          <button type="button" className="btn" onClick={() => setPickerOpen(true)}>
            {t('admin.browse')}
          </button>
        </div>
        <button className="btn primary">{t('admin.addLibrary')}</button>
      </form>
      <p className="muted hint">{t('admin.browseHint')}</p>

      {pickerOpen && (
        <PathPicker
          initialPath={path}
          onPick={(p) => {
            setPath(p);
            setPickerOpen(false);
          }}
          onClose={() => setPickerOpen(false)}
        />
      )}

      {libs.length === 0 ? (
        <div className="empty">{t('admin.libEmpty')}</div>
      ) : (
        <div className="playlist-list">
          {libs.map((l) => (
            <div key={l.id} className="card playlist-card">
              <div className="playlist-main">
                <div className="playlist-icon">📁</div>
                <div>
                  <div className="playlist-name">
                    {l.name}
                    {l.blocked && <span className="status-badge status-error">{t('admin.libBlockedBadge')}</span>}
                    <span className={`status-badge status-${l.scan_status}`}>
                      {l.scan_status === 'scanning'
                        ? t('admin.statusScanning')
                        : l.scan_status === 'error'
                          ? t('admin.statusError')
                          : l.scan_status === 'cancelled'
                            ? t('admin.statusCancelled')
                            : t('admin.statusIdle')}
                    </span>
                  </div>
                  <div className="muted mono">{l.path}</div>
                  <div className="muted">
                    {l.scan_status === 'scanning'
                      ? t('admin.scanningProgress', {
                          count: l.video_count,
                          time: new Date(l.scan_started_at).toLocaleTimeString(i18n.language),
                        })
                      : t('admin.videos', { count: l.video_count })}
                  </div>
                  {l.scan_error && <div className="form-error small">{l.scan_error}</div>}
                </div>
              </div>
              <div className="playlist-item-actions">
                {l.scan_status === 'scanning' ? (
                  <button className="btn small danger-ghost" onClick={() => cancelScan(l.id)}>
                    {t('admin.stopScan')}
                  </button>
                ) : (
                  <button className="btn small" onClick={() => scan(l.id)}>
                    {t('admin.scan')}
                  </button>
                )}
                <button className="btn small" onClick={() => openFolder(l)}>
                  📂 {t('admin.openFolder')}
                </button>
                <button className="btn small danger-ghost" onClick={() => toggleLibraryBlock(l)}>
                  {l.blocked ? t('admin.blockUnblock') : t('admin.blockLibrary')}
                </button>
                <button className="btn small danger-ghost" onClick={() => remove(l.id)}>{t('admin.delete')}</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function VideoAdmin() {
  const { t } = useTranslation();
  const [videos, setVideos] = useState([]);
  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState(null);
  const [form, setForm] = useState(null);
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');
  const [scrapingId, setScrapingId] = useState(null);

  useEffect(() => {
    const params = new URLSearchParams({ page: String(page), page_size: '50', sort: 'added_desc', include_blocked: '1' });
    if (search) params.set('q', search);
    api(`/videos?${params}`)
      .then((d) => {
        setVideos(d.items);
        setTotal(d.total);
      })
      .catch((e) => setErr(e.message));
  }, [page, search]);

  function submit(e) {
    e.preventDefault();
    setPage(1);
    setSearch(q.trim());
  }

  async function save(e) {
    e.preventDefault();
    try {
      await api(`/videos/${editing.id}`, {
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
      setMsg(`${form.title} — ${t('video.metaUpdated')}`);
      setEditing(null);
      setPage(1);
      setSearch(search);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function uploadPoster(e) {
    e.preventDefault();
    const file = e.target.poster.files[0];
    if (!file) return;
    const fd = new FormData();
    fd.append('poster', file);
    try {
      await api(`/videos/${editing.id}/poster`, { method: 'POST', form: fd });
      setMsg(t('admin.posterUpdated'));
      setEditing(null);
      setSearch(search);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function scrape(v) {
    setScrapingId(v.id);
    setErr('');
    try {
      await api(`/videos/${v.id}/scrape`, { method: 'POST' });
      setMsg(t('admin.scrapeDone', { title: v.title }));
      setPage(1);
      setSearch(search);
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setScrapingId(null);
    }
  }

  async function toggleBlock(v) {
    setErr('');
    try {
      if (v.blocked) {
        await api(`/admin/blocked-titles/${v.blocked_id}`, { method: 'DELETE' });
        setMsg(t('admin.blockRemoved', { title: v.title }));
      } else {
        await api('/admin/blocked-titles', { method: 'POST', body: { title: v.title } });
        setMsg(t('admin.blockAdded', { title: v.title }));
      }
      setPage(1);
      setSearch(search);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="search-form" onSubmit={submit}>
        <input className="search-input" placeholder={t('admin.searchVideos')} value={q} onChange={(e) => setQ(e.target.value)} />
        <button className="btn primary">{t('common.search')}</button>
        <span className="muted">{t('admin.results', { count: total })}</span>
      </form>

      <div className="admin-video-list">
        {videos.map((v) => (
          <div key={v.id} className="card admin-video-row">
            <div className="mono muted" style={{ flex: 1, minWidth: 0 }}>
              <div className="ellipsis">
                {v.title}
                {v.blocked && <span className="status-badge status-error">{t('admin.blockedBadge')}</span>}
              </div>
              <div className="small-muted">{v.filename}</div>
            </div>
            <button
              className="btn small danger-ghost"
              onClick={() => toggleBlock(v)}
            >
              {v.blocked ? t('admin.blockUnblock') : t('admin.blockThis')}
            </button>
            <button
              className="btn small"
              onClick={() => {
                setEditing(v);
                setForm({
                  title: v.title,
                  synopsis: v.synopsis,
                  year: v.year || '',
                  genres: (v.genres || []).join(', '),
                });
              }}
            >
              {t('common.edit')}
            </button>
            <button className="btn small" onClick={() => scrape(v)} disabled={scrapingId === v.id}>
              {scrapingId === v.id ? t('admin.scraping') : t('admin.scrape')}
            </button>
          </div>
        ))}
      </div>
      {videos.length < total && (
        <div className="load-more">
          <button className="btn" onClick={() => setPage((p) => p + 1)}>{t('admin.loadMore')}</button>
        </div>
      )}

      {editing && (
        <div className="modal-backdrop" onClick={() => setEditing(null)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t('video.editTitle')}: {editing.title}</h3>
            <form onSubmit={save}>
              <label>
                {t('video.fieldTitle')}
                <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
              </label>
              <label>
                {t('video.fieldYear')}
                <input type="number" value={form.year} onChange={(e) => setForm({ ...form, year: e.target.value })} />
              </label>
              <label>
                {t('video.fieldGenres')}
                <input value={form.genres} onChange={(e) => setForm({ ...form, genres: e.target.value })} />
              </label>
              <label>
                {t('video.fieldSynopsis')}
                <textarea rows={4} value={form.synopsis} onChange={(e) => setForm({ ...form, synopsis: e.target.value })} />
              </label>
              <div className="modal-actions">
                <button className="btn primary">{t('common.save')}</button>
                <button type="button" className="btn ghost" onClick={() => setEditing(null)}>{t('common.cancel')}</button>
              </div>
            </form>
            <form onSubmit={uploadPoster} className="upload-poster">
              <label className="btn small">{t('admin.uploadPoster')} <input type="file" name="poster" accept="image/jpeg,image/png,image/webp" hidden /></label>
              <button className="btn small primary" type="submit">{t('admin.savePoster')}</button>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function Users() {
  const { t } = useTranslation();
  const { user: me } = useAuth();
  const [users, setUsers] = useState([]);
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  function refresh() {
    api('/admin/users')
      .then((d) => setUsers(d.items))
      .catch((e) => setErr(e.message));
  }
  useEffect(refresh, []);

  async function setRole(u, role) {
    try {
      await api(`/admin/users/${u.id}`, { method: 'PATCH', body: { role } });
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  async function resetPassword(u) {
    const pwd = window.prompt(t('admin.resetPwdPrompt', { name: u.username }));
    if (!pwd) return;
    try {
      await api(`/admin/users/${u.id}/reset-password`, { method: 'POST', body: { password: pwd } });
      setMsg(t('admin.resetPwdDone', { name: u.username }));
    } catch (e) {
      setErr(e.message);
    }
  }

  async function remove(u) {
    if (!window.confirm(t('admin.deleteUserConfirm', { name: u.username }))) return;
    try {
      await api(`/admin/users/${u.id}`, { method: 'DELETE' });
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <div className="playlist-list">
        {users.map((u) => (
          <div key={u.id} className="card playlist-card">
            <div className="playlist-main">
              <div className="playlist-icon">👤</div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="playlist-name">
                  {u.display_name || u.username}
                  {u.id === me?.id && <span className="status-badge status-idle">{t('admin.currentAccount')}</span>}
                </div>
                <div className="muted">@{u.username}</div>
              </div>
              <select value={u.role} onChange={(e) => setRole(u, e.target.value)}>
                <option value="user">{t('admin.roleUser')}</option>
                <option value="admin">{t('admin.roleAdmin')}</option>
              </select>
            </div>
            <div className="playlist-item-actions">
              <button className="btn small" onClick={() => resetPassword(u)}>
                {t('admin.resetPassword')}
              </button>
              <button
                className="btn small danger-ghost"
                onClick={() => remove(u)}
                disabled={u.id === me?.id}
              >
                {t('admin.deleteUser')}
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
