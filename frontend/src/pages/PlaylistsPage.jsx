import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

export default function PlaylistsPage() {
  const { t } = useTranslation();
  const [playlists, setPlaylists] = useState([]);
  const [name, setName] = useState('');
  const [desc, setDesc] = useState('');
  const [err, setErr] = useState('');
  const [msg, setMsg] = useState('');

  useEffect(() => {
    refresh();
  }, []);

  function refresh() {
    api('/playlists').then((d) => setPlaylists(d.items)).catch((e) => setErr(e.message));
  }

  async function create(e) {
    e.preventDefault();
    try {
      await api('/playlists', { method: 'POST', body: { name, description: desc } });
      setName('');
      setDesc('');
      setMsg(t('playlists.created'));
      refresh();
    } catch (e2) {
      setErr(e2.message);
    }
  }

  async function remove(id) {
    if (!window.confirm(t('playlists.deleteConfirm'))) return;
    try {
      await api(`/playlists/${id}`, { method: 'DELETE' });
      refresh();
    } catch (e) {
      setErr(e.message);
    }
  }

  return (
    <div className="container">
      <h1>{t('playlists.title')}</h1>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}

      <form className="card inline-form" onSubmit={create}>
        <input
          placeholder={t('playlists.namePlaceholder')}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <input
          placeholder={t('playlists.descPlaceholder')}
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
        />
        <button className="btn primary">{t('playlists.create')}</button>
      </form>

      {playlists.length === 0 ? (
        <div className="empty">{t('playlists.empty')}</div>
      ) : (
        <div className="playlist-list">
          {playlists.map((p) => (
            <div key={p.id} className="card playlist-card">
              <Link to={`/playlists/${p.id}`} className="playlist-main">
                <div className="playlist-icon">▶</div>
                <div>
                  <div className="playlist-name">{p.name}</div>
                  <div className="muted">
                    {t('playlists.videos', { count: p.item_count })}
                    {p.description ? ` · ${p.description}` : ''}
                  </div>
                </div>
              </Link>
              <button className="btn danger-ghost" onClick={() => remove(p.id)}>
                {t('common.delete')}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
