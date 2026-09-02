import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';
import { useAuth } from '../auth.jsx';
import PathPicker from '../components/PathPicker.jsx';
import { fmtBytes } from '../i18n';
import BlockedAdmin from './BlockedAdmin.jsx';
import UploadsAdmin from './UploadsAdmin.jsx';
import DownloadsAdmin from './DownloadsAdmin.jsx';
import LiveAdmin from './LiveAdmin.jsx';
import IptvAdmin from './IptvAdmin.jsx';
import { AdminRequests } from './RequestsPage.jsx';
import QualityProfiles from './QualityProfiles.jsx';
import Invites from './Invites.jsx';
import RecordingsAdmin from './RecordingsAdmin.jsx';
import TraktAdmin from './TraktAdmin.jsx';
import ModerationAdmin from './ModerationAdmin.jsx';
import StorageAdmin from './StorageAdmin.jsx';
import JobsAdmin from './JobsAdmin.jsx';
import WebhooksAdmin from './WebhooksAdmin.jsx';

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
        <button className={tab === 'uploads' ? 'tab active' : 'tab'} onClick={() => setTab('uploads')}>
          {t('admin.tabUploads')}
        </button>
        <button className={tab === 'downloads' ? 'tab active' : 'tab'} onClick={() => setTab('downloads')}>
          {t('admin.tabDownloads')}
        </button>
        <button className={tab === 'live' ? 'tab active' : 'tab'} onClick={() => setTab('live')}>
          {t('admin.tabLive')}
        </button>
        <button className={tab === 'storage' ? 'tab active' : 'tab'} onClick={() => setTab('storage')}>
          {t('admin.tabStorage')}
        </button>
        <button className={tab === 'jobs' ? 'tab active' : 'tab'} onClick={() => setTab('jobs')}>
          {t('admin.tabJobs')}
        </button>
        <button className={tab === 'webhooks' ? 'tab active' : 'tab'} onClick={() => setTab('webhooks')}>
          {t('admin.tabWebhooks')}
        </button>
        <button className={tab === 'iptv' ? 'tab active' : 'tab'} onClick={() => setTab('iptv')}>
          {t('admin.tabIptv')}
        </button>
        <button className={tab === 'requests' ? 'tab active' : 'tab'} onClick={() => setTab('requests')}>
          {t('admin.tabRequests')}
        </button>
        <button className={tab === 'quality' ? 'tab active' : 'tab'} onClick={() => setTab('quality')}>
          {t('admin.tabQuality')}
        </button>
        <button className={tab === 'invites' ? 'tab active' : 'tab'} onClick={() => setTab('invites')}>
          {t('admin.tabInvites')}
        </button>
        <button className={tab === 'recordings' ? 'tab active' : 'tab'} onClick={() => setTab('recordings')}>
          {t('admin.tabRecordings')}
        </button>
        <button className={tab === 'trakt' ? 'tab active' : 'tab'} onClick={() => setTab('trakt')}>
          {t('admin.tabTrakt')}
        </button>
        <button className={tab === 'moderation' ? 'tab active' : 'tab'} onClick={() => setTab('moderation')}>
          {t('admin.tabModeration')}
        </button>
      </div>
      {tab === 'overview' && <Overview />}
      {tab === 'libraries' && <Libraries />}
      {tab === 'videos' && <VideoAdmin />}
      {tab === 'blocked' && <BlockedAdmin />}
      {tab === 'users' && <Users />}
      {tab === 'uploads' && <UploadsAdmin />}
      {tab === 'downloads' && <DownloadsAdmin />}
      {tab === 'live' && <LiveAdmin />}
      {tab === 'storage' && <StorageAdmin />}
      {tab === 'jobs' && <JobsAdmin />}
      {tab === 'webhooks' && <WebhooksAdmin />}
      {tab === 'iptv' && <IptvAdmin />}
      {tab === 'requests' && <AdminRequests />}
      {tab === 'quality' && <QualityProfiles />}
      {tab === 'invites' && <Invites />}
      {tab === 'recordings' && <RecordingsAdmin />}
      {tab === 'trakt' && <TraktAdmin />}
      {tab === 'moderation' && <ModerationAdmin />}
    </div>
  );
}

function Overview() {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);
  const [importMsg, setImportMsg] = useState('');
  const [importErr, setImportErr] = useState('');
  const [backups, setBackups] = useState([]);
  const [maintMsg, setMaintMsg] = useState('');
  const [maintErr, setMaintErr] = useState('');
  useEffect(() => {
    api('/admin/stats').then(setStats).catch(() => {});
  }, []);
  async function importBackup(e) {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setImportMsg('');
    setImportErr('');
    try {
      const fd = new FormData();
      fd.append('backup', file);
      const d = await api('/admin/import', { method: 'POST', form: fd });
      setImportMsg(t('admin.importDone', { counts: JSON.stringify(d.counts || {}) }));
      api('/admin/stats').then(setStats).catch(() => {});
    } catch (e2) {
      setImportErr(e2.message);
    }
  }

  async function testNotify() {
    try {
      await api('/admin/notify/test', { method: 'POST' });
      setImportMsg(t('admin.notifySent'));
    } catch (e) {
      setImportErr(e.message);
    }
  }

  async function refreshBackups() {
    try {
      const d = await api('/admin/backups');
      setBackups(d.items || []);
    } catch (e) {
      setMaintErr(e.message);
    }
  }
  useEffect(() => {
    refreshBackups();
  }, []);

  async function runMaintenance() {
    setMaintErr('');
    setMaintMsg('');
    try {
      const d = await api('/admin/maintenance/run', { method: 'POST' });
      setMaintMsg(t('admin.maintenanceDone', { backup: d.backup }));
      refreshBackups();
    } catch (e) {
      setMaintErr(e.message);
    }
  }
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
      <div className="detail-actions">
        <a className="btn ghost" href={mediaUrl('/openapi.json')} target="_blank" rel="noreferrer">
          {t('admin.apiDocs')}
        </a>
        <button className="btn ghost" onClick={runMaintenance}>{t('admin.runMaintenance')}</button>
        <button className="btn ghost" onClick={testNotify}>{t('admin.testNotify')}</button>
        <a className="btn ghost" href={mediaUrl('/admin/export')}>
          {t('admin.export')}
        </a>
        <label className="btn ghost">
          {t('admin.import')}
          <input type="file" accept=".json,application/json" hidden onChange={importBackup} />
        </label>
      </div>
      {maintMsg && <div className="toast toast-success">{maintMsg}</div>}
      {maintErr && <div className="form-error">{maintErr}</div>}
      {backups.length > 0 && (
        <div className="card backups-box">
          <h3>{t('admin.backups')}</h3>
          {backups.map((b) => (
            <div key={b.name} className="admin-video-row trash-row">
              <div className="mono muted" style={{ flex: 1, minWidth: 0 }}>
                <div className="ellipsis">{b.name}</div>
                <div className="small-muted">{fmtBytes(b.size)} · {new Date(b.created_at).toLocaleString()}</div>
              </div>
              <a className="btn small" href={mediaUrl(`/admin/backups/${encodeURIComponent(b.name)}`)}>{t('common.download')}</a>
            </div>
          ))}
        </div>
      )}
      {importMsg && <div className="toast toast-success">{importMsg}</div>}
      {importErr && <div className="form-error">{importErr}</div>}
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
  const [healthReports, setHealthReports] = useState({});
  const [healthBusy, setHealthBusy] = useState(new Set());

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

  async function runHealth(l) {
    setHealthBusy((prev) => new Set(prev).add(l.id));
    setErr('');
    try {
      const d = await api(`/libraries/${l.id}/health`, { method: 'POST' });
      setHealthReports((prev) => ({ ...prev, [l.id]: d }));
    } catch (e) {
      setErr(e.message);
    } finally {
      setHealthBusy((prev) => {
        const next = new Set(prev);
        next.delete(l.id);
        return next;
      });
    }
  }

  async function keepBest(l) {
    setErr('');
    try {
      const d = await api(`/libraries/${l.id}/health/keep-best`, { method: 'POST' });
      setMsg(t('admin.healthMoved', { count: (d.moved || []).length }));
      await runHealth(l);
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  async function nfo(l, action) {
    setErr('');
    try {
      const d = await api(`/libraries/${l.id}/${action === 'export' ? 'export-nfo' : 'import-nfo'}`, { method: 'POST' });
      setMsg(action === 'export'
        ? t('admin.nfoExported', { count: d.exported || 0 })
        : t('admin.nfoImported', { count: d.updated || 0 }));
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  async function setQuota(l, val) {
    const quota = parseInt(val, 10) || 0;
    setErr('');
    try {
      await api(`/libraries/${l.id}`, { method: 'PATCH', body: { quota_bytes: quota } });
      setMsg(t('admin.quotaSaved'));
      refresh();
    } catch (e) {
      setErr(e.message);
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
                <button className="btn small" onClick={() => runHealth(l)} disabled={healthBusy.has(l.id)}>
                  {healthBusy.has(l.id) ? t('admin.healthRunning') : t('admin.healthCheck')}
                </button>
                <button className="btn small" onClick={() => nfo(l, 'export')}>{t('admin.exportNFO')}</button>
                <button className="btn small" onClick={() => nfo(l, 'import')}>{t('admin.importNFO')}</button>
                <input
                  className="quota-input"
                  type="number"
                  min="0"
                  placeholder={t('admin.quota')}
                  defaultValue={l.quota_bytes || ''}
                  onBlur={(e) => setQuota(l, e.target.value)}
                />
                <button className="btn small danger-ghost" onClick={() => toggleLibraryBlock(l)}>
                  {l.blocked ? t('admin.blockUnblock') : t('admin.blockLibrary')}
                </button>
                <button className="btn small danger-ghost" onClick={() => remove(l.id)}>{t('admin.delete')}</button>
              </div>
              {healthReports[l.id] && (
                <div className="health-report">
                  <div className="muted small">{t('admin.healthChecked', { count: healthReports[l.id].checked })}</div>
                  {healthReports[l.id].missing.length > 0 && (
                    <div className="form-error small">
                      {t('admin.healthMissing', { n: healthReports[l.id].missing.length })}:{' '}
                      {healthReports[l.id].missing.join(', ')}
                    </div>
                  )}
                  {healthReports[l.id].corrupt.length > 0 && (
                    <div className="form-error small">
                      {t('admin.healthCorrupt', { n: healthReports[l.id].corrupt.length })}:{' '}
                      {healthReports[l.id].corrupt.join(', ')}
                    </div>
                  )}
                  {healthReports[l.id].duplicates.map((g) => (
                    <div key={g.size} className="muted small">
                      {t('admin.healthDuplicates', { size: fmtBytes(g.size), count: g.count })}:{' '}
                      {g.files.join(', ')}
                    </div>
                  ))}
                  {healthReports[l.id].duplicates.length > 0 && (
                    <button className="btn small ghost" onClick={() => keepBest(l)}>
                      {t('admin.healthKeepBest')}
                    </button>
                  )}
                  {healthReports[l.id].missing.length === 0 &&
                    healthReports[l.id].corrupt.length === 0 &&
                    healthReports[l.id].duplicates.length === 0 && (
                      <div className="muted small">{t('admin.healthOk')}</div>
                    )}
                </div>
              )}
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
  const [selected, setSelected] = useState(new Set());
  const [batchTag, setBatchTag] = useState('');
  const [batchBusy, setBatchBusy] = useState(false);
  const [trashItems, setTrashItems] = useState([]);
  const [showTrash, setShowTrash] = useState(false);

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
          content_rating: form.content_rating,
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

  function toggleSelect(id) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function batchAction(action) {
    if (selected.size === 0) return;
    setBatchBusy(true);
    setErr('');
    try {
      const body = { ids: [...selected], action };
      if (action === 'tag') {
        const tag = batchTag.trim().toLowerCase();
        if (!tag) {
          setErr(t('admin.batchTagRequired'));
          return;
        }
        body.tag = tag;
      }
      await api('/admin/videos/batch', { method: 'POST', body });
      setSelected(new Set());
      setBatchTag('');
      setMsg(t('admin.batchDone'));
      setSearch(search);
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBatchBusy(false);
    }
  }

  async function refreshTrash() {
    try {
      const d = await api('/admin/trash');
      setTrashItems(d.items || []);
      setShowTrash(true);
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function restoreTrash(id) {
    setErr('');
    try {
      const d = await api(`/admin/trash/${id}/restore`, { method: 'POST' });
      setMsg(t('admin.restored', { path: d.restored }));
      setTrashItems((prev) => prev.filter((t) => t.id !== id));
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

      {selected.size > 0 && (
        <div className="browse-toolbar browse-save">
          <span className="muted">{t('admin.batchSelected', { count: selected.size })}</span>
          <input
            className="collection-input"
            placeholder={t('admin.batchTagPlaceholder')}
            value={batchTag}
            onChange={(e) => setBatchTag(e.target.value)}
          />
          <button className="btn small ghost" disabled={batchBusy} onClick={() => batchAction('tag')}>
            {t('admin.batchApplyTag')}
          </button>
          <button className="btn small ghost" disabled={batchBusy} onClick={() => batchAction('clear_tags')}>
            {t('admin.batchClearTags')}
          </button>
          <button className="btn small danger-ghost" disabled={batchBusy} onClick={() => batchAction('delete')}>
            {t('admin.batchDelete')}
          </button>
          <button className="btn small ghost" onClick={() => setSelected(new Set())}>{t('common.clear')}</button>
        </div>
      )}
      <div className="browse-toolbar browse-save">
        <button className="btn small ghost" onClick={refreshTrash}>{t('admin.trash')}</button>
      </div>

      {showTrash && (
        <div className="card trash-box">
          <h3>{t('admin.trash')}</h3>
          {trashItems.length === 0 ? (
            <div className="empty">{t('admin.trashEmpty')}</div>
          ) : (
            trashItems.map((item) => (
              <div key={item.id} className="admin-video-row trash-row">
                <div className="mono muted" style={{ flex: 1, minWidth: 0 }}>
                  <div className="ellipsis">{item.original_path}</div>
                  <div className="small-muted">{new Date(item.moved_at).toLocaleString()}</div>
                </div>
                <button className="btn small" onClick={() => restoreTrash(item.id)}>{t('admin.restore')}</button>
              </div>
            ))
          )}
        </div>
      )}

      <div className="admin-video-list">
        {videos.map((v) => (
          <div key={v.id} className="card admin-video-row">
            <input
              type="checkbox"
              checked={selected.has(v.id)}
              onChange={() => toggleSelect(v.id)}
            />
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
                  content_rating: v.content_rating || '',
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
                {t('video.fieldContentRating')}
                <input value={form.content_rating} onChange={(e) => setForm({ ...form, content_rating: e.target.value })} />
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

  async function setRating(u, val) {
    try {
      await api(`/admin/users/${u.id}`, { method: 'PATCH', body: { allowed_rating: val.trim() } });
      setMsg(t('admin.ratingSaved', { name: u.username }));
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
              <input
                className="rating-input"
                placeholder={t('admin.allowedRating')}
                defaultValue={u.allowed_rating || ''}
                onBlur={(e) => setRating(u, e.target.value)}
              />
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
