import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, apiBaseUrl, getToken } from '../api.js';

export default function IptvAdmin() {
  const { t } = useTranslation();
  const [channels, setChannels] = useState([]);
  const [libraries, setLibraries] = useState([]);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');

  const [name, setName] = useState('');
  const [sourceUrl, setSourceUrl] = useState('');
  const [tvgId, setTvgId] = useState('');
  const [tvgName, setTvgName] = useState('');
  const [logo, setLogo] = useState('');
  const [group, setGroup] = useState('');
  const [importUrl, setImportUrl] = useState('');
  const [libId, setLibId] = useState('');
  const [libName, setLibName] = useState('');
  const [libGroup, setLibGroup] = useState('');
  const [epgUrl, setEpgUrl] = useState('');
  const [busy, setBusy] = useState(false);

  function load() {
    api('/iptv/channels').then((d) => setChannels(d.items || [])).catch((e) => setErr(e.message));
    api('/libraries').then((d) => setLibraries(d.items || [])).catch(() => {});
  }
  useEffect(load, []);

  async function run(fn, successKey) {
    setBusy(true);
    setErr('');
    setMsg('');
    try {
      await fn();
      setMsg(t(successKey));
      load();
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  const token = getToken() || '';
  const m3uUrl = `${apiBaseUrl()}/api/iptv/channels.m3u?token=${encodeURIComponent(token)}`;
  const epgUrlOut = `${apiBaseUrl()}/api/iptv/epg.xml?token=${encodeURIComponent(token)}`;

  return (
    <div>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}

      <div className="card admin-tools">
        <div className="admin-tools-head">{t('iptv.addChannel')}</div>
        <div className="field-row">
          <input placeholder={t('iptv.name')} value={name} onChange={(e) => setName(e.target.value)} />
          <input placeholder={t('iptv.sourceUrl')} value={sourceUrl} onChange={(e) => setSourceUrl(e.target.value)} />
          <button
            className="btn small primary"
            disabled={busy}
            onClick={() => run(async () => {
              await api('/iptv/channels', {
                method: 'POST',
                body: { name, source_url: sourceUrl, tvg_id: tvgId, tvg_name: tvgName, logo, group_title: group },
              });
              setName(''); setSourceUrl(''); setTvgId(''); setTvgName(''); setLogo(''); setGroup('');
            }, 'iptv.added')}
          >
            {t('iptv.add')}
          </button>
        </div>
        <div className="field-row">
          <input placeholder="tvg-id" value={tvgId} onChange={(e) => setTvgId(e.target.value)} />
          <input placeholder="tvg-name" value={tvgName} onChange={(e) => setTvgName(e.target.value)} />
          <input placeholder={t('iptv.logo')} value={logo} onChange={(e) => setLogo(e.target.value)} />
          <input placeholder={t('iptv.group')} value={group} onChange={(e) => setGroup(e.target.value)} />
        </div>
      </div>

      <div className="card admin-tools">
        <div className="admin-tools-head">{t('iptv.importM3U')}</div>
        <div className="field-row">
          <input placeholder={t('iptv.m3uUrl')} value={importUrl} onChange={(e) => setImportUrl(e.target.value)} />
          <button
            className="btn small primary"
            disabled={busy}
            onClick={() => run(async () => {
              await api('/iptv/import', { method: 'POST', body: { url: importUrl } });
              setImportUrl('');
            }, 'iptv.imported')}
          >
            {t('iptv.import')}
          </button>
        </div>
      </div>

      <div className="card admin-tools">
        <div className="admin-tools-head">{t('iptv.libraryChannel')}</div>
        <div className="field-row">
          <select value={libId} onChange={(e) => setLibId(e.target.value)}>
            <option value="">{t('iptv.library')}</option>
            {libraries.map((l) => (
              <option key={l.id} value={l.id}>{l.name}</option>
            ))}
          </select>
          <input placeholder={t('iptv.name')} value={libName} onChange={(e) => setLibName(e.target.value)} />
          <input placeholder={t('iptv.group')} value={libGroup} onChange={(e) => setLibGroup(e.target.value)} />
          <button
            className="btn small primary"
            disabled={busy || !libId || !libName}
            onClick={() => run(async () => {
              await api('/iptv/library-channel', {
                method: 'POST',
                body: { library_id: libId, name: libName, group_title: libGroup },
              });
              setLibName(''); setLibGroup('');
            }, 'iptv.added')}
          >
            {t('iptv.create')}
          </button>
        </div>
      </div>

      <div className="card admin-tools">
        <div className="admin-tools-head">{t('iptv.epgImport')}</div>
        <div className="field-row">
          <input placeholder={t('iptv.epgUrl')} value={epgUrl} onChange={(e) => setEpgUrl(e.target.value)} />
          <button
            className="btn small primary"
            disabled={busy}
            onClick={() => run(async () => {
              await api('/iptv/epg/import', { method: 'POST', body: { url: epgUrl } });
              setEpgUrl('');
            }, 'iptv.imported')}
          >
            {t('iptv.import')}
          </button>
        </div>
        <div className="field-row">
          <span className="muted small">{t('iptv.m3uOutput')}</span>
          <code className="iptv-url">{m3uUrl}</code>
        </div>
        <div className="field-row">
          <span className="muted small">{t('iptv.epgOutput')}</span>
          <code className="iptv-url">{epgUrlOut}</code>
        </div>
      </div>

      <div className="card">
        <h3>{t('iptv.channels')}</h3>
        {channels.length === 0 && <div className="empty">{t('iptv.noChannels')}</div>}
        {channels.map((c) => (
          <div key={c.id} className="playlist-row">
            <div className="playlist-item-main">
              <b>{c.name}</b>
              <span className="muted small">
                {[c.group_title, c.tvg_id, c.library_id ? t('iptv.libraryChannel') : c.source_url]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </div>
            <button
              className="btn small ghost"
              onClick={() => run(async () => {
                await api(`/iptv/channels/${c.id}`, { method: 'DELETE' });
              }, 'iptv.deleted')}
            >
              {t('common.remove')}
            </button>
            <label className="btn small ghost">
              {t('iptv.logoUpload')}
              <input
                type="file"
                accept="image/jpeg,image/png,image/webp"
                hidden
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  e.target.value = '';
                  if (!file) return;
                  const fd = new FormData();
                  fd.append('logo', file);
                  await run(async () => {
                    await api(`/iptv/channels/${c.id}/logo`, { method: 'POST', form: fd });
                  }, 'iptv.logoUpdated');
                }}
              />
            </label>
          </div>
        ))}
      </div>
    </div>
  );
}
