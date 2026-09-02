import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';

const EVENT_KEYS = ['scan', 'download', 'comment', 'favorite', 'rating', 'subscription', 'new_episode'];

export default function SettingsPage() {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(true);
  const [events, setEvents] = useState([]);
  const [msg, setMsg] = useState('');
  const [err, setErr] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api('/users/me/notification-prefs')
      .then((d) => {
        setEnabled(d.enabled !== false);
        setEvents(d.events || []);
      })
      .catch(() => {});
  }, []);

  function toggleEvent(e) {
    const value = e.target.value;
    setEvents((prev) => (prev.includes(value) ? prev.filter((x) => x !== value) : [...prev, value]));
  }

  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setErr('');
    setMsg('');
    try {
      await api('/users/me/notification-prefs', {
        method: 'PUT',
        body: { enabled, events },
      });
      setMsg(t('settings.saved'));
    } catch (e2) {
      setErr(e2.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="container">
      <h1>{t('nav.settings')}</h1>
      {msg && <div className="toast toast-success">{msg}</div>}
      {err && <div className="form-error">{err}</div>}
      <form className="card admin-tools" onSubmit={save}>
        <div className="admin-tools-head">{t('settings.notifications')}</div>
        <label className="scrape-force">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          {t('settings.enabled')}
        </label>
        <div className="field-row">
          {EVENT_KEYS.map((k) => (
            <label key={k} className="scrape-force">
              <input type="checkbox" value={k} checked={events.includes(k)} onChange={toggleEvent} />
              {t(`settings.event${k.charAt(0).toUpperCase()}${k.slice(1)}`)}
            </label>
          ))}
        </div>
        <button className="btn primary" disabled={busy}>{t('common.save')}</button>
      </form>
    </div>
  );
}
