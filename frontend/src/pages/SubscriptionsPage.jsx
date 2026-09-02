import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import SeriesCard from '../components/SeriesCard.jsx';

export default function SubscriptionsPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/users/me/subscriptions')
      .then((d) => setItems(d.items || []))
      .catch((e) => setErr(e.message));
  }, []);

  return (
    <div className="container">
      <h1>{t('nav.subscriptions')}</h1>
      {err && <div className="form-error">{err}</div>}
      {items.length === 0 && !err && <div className="empty">{t('subscriptions.empty')}</div>}
      <div className="video-grid">
        {items.map((s) => (
          <SeriesCard key={s.id} series={s} />
        ))}
      </div>
    </div>
  );
}
