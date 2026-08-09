import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import SeriesCard from '../components/SeriesCard.jsx';

export default function SeriesListPage() {
  const { t } = useTranslation();
  const [series, setSeries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/series')
      .then((d) => setSeries(d.items))
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="container">
      <h1>{t('series.title')}</h1>
      {err && <div className="form-error">{err}</div>}
      {loading ? (
        <div className="loading">{t('common.loading')}</div>
      ) : series.length === 0 ? (
        <div className="empty">{t('series.empty')}</div>
      ) : (
        <div className="card-grid">
          {series.map((s) => (
            <SeriesCard key={s.id} series={s} />
          ))}
        </div>
      )}
    </div>
  );
}

