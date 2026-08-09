import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import SeriesCard from '../components/SeriesCard.jsx';
import PathFilterModal from '../components/PathFilterModal.jsx';

export default function SeriesListPage() {
  const { t } = useTranslation();
  const [series, setSeries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState('');
  const [showFilter, setShowFilter] = useState(false);
  const [libraries, setLibraries] = useState([]);
  const [libraryId, setLibraryId] = useState('');

  function loadSeries() {
    setLoading(true);
    const params = new URLSearchParams();
    if (libraryId) params.set('library_id', libraryId);
    api(`/series?${params}`)
      .then((d) => setSeries(d.items))
      .catch((e) => setErr(e.message))
      .finally(() => setLoading(false));
  }
  useEffect(loadSeries, [libraryId]);
  useEffect(() => {
    api('/libraries').then((d) => setLibraries(d.items)).catch(() => {});
  }, []);

  return (
    <div className="container">
      <div className="section-head">
        <h1>{t('series.title')}</h1>
        <div className="browse-toolbar" style={{ marginBottom: 0 }}>
          <select value={libraryId} onChange={(e) => setLibraryId(e.target.value)}>
            <option value="">{t('browse.allLibraries')}</option>
            {libraries.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
          <button className="btn" onClick={() => setShowFilter(true)}>
            {t('series.pathFilter')}
          </button>
        </div>
      </div>
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
      {showFilter && (
        <PathFilterModal
          onClose={() => setShowFilter(false)}
          onChanged={loadSeries}
        />
      )}
    </div>
  );
}
