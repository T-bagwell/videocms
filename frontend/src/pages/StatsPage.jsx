import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

function fmtHours(sec) {
  const h = Math.floor(sec / 3600);
  const m = Math.round((sec % 3600) / 60);
  return `${h}h ${m}m`;
}

export default function StatsPage() {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/users/me/stats').then(setStats).catch((e) => setErr(e.message));
  }, []);

  if (err) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!stats) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  const maxDay = Math.max(1, ...(stats.weekly || []).map((d) => d.seconds));
  return (
    <div className="container">
      <div className="reader-head">
        <h1>{t('nav.stats')}</h1>
        <a className="btn small ghost" href={mediaUrl('/users/me/stats/export')}>
          {t('stats.export')}
        </a>
      </div>
      <div className="stats-grid">
        <div className="card stat"><div className="stat-num">{fmtHours(stats.total_watch_sec)}</div><div>{t('stats.totalTime')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.plays}</div><div>{t('stats.plays')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.movies_watched}</div><div>{t('stats.movies')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.episodes_watched}</div><div>{t('stats.episodes')}</div></div>
        <div className="card stat"><div className="stat-num">{stats.days_active}</div><div>{t('stats.daysActive')}</div></div>
      </div>

      <div className="card">
        <h3>{t('stats.weekly')}</h3>
        <div className="weekly-bars">
          {[...(stats.weekly || [])].reverse().map((d) => (
            <div key={d.date} className="weekly-col">
              <div className="weekly-bar" style={{ height: `${Math.max(4, (d.seconds / maxDay) * 120)}px` }} title={`${d.date}: ${Math.round(d.seconds / 60)}m`} />
              <span className="weekly-label">{d.date.slice(5)}</span>
            </div>
          ))}
        </div>
      </div>

      {stats.top_genres?.length > 0 && (
        <div className="card">
          <h3>{t('stats.genres')}</h3>
          <div className="tag-box">
            {stats.top_genres.map((g) => (
              <span key={g.name} className="tag-chip">
                {g.name} · {g.count}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
