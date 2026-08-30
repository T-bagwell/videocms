import { useEffect, useRef, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../api.js';
import VideoCard from '../components/VideoCard.jsx';
import SeriesCard from '../components/SeriesCard.jsx';
import PathFilterModal from '../components/PathFilterModal.jsx';
import { useAuth } from '../auth.jsx';
import { useTranslation } from 'react-i18next';

const PAGE_SIZE = 24;

export default function BrowsePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const { user } = useAuth();
  const { t } = useTranslation();
  const [videos, setVideos] = useState([]);
  const [continueWatching, setContinueWatching] = useState([]);
  const [libraries, setLibraries] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  const [libraryId, setLibraryId] = useState('');
  const [sort, setSort] = useState('title');
  const [vtype, setVtype] = useState('');
  const [tag, setTag] = useState(searchParams.get('tag') || '');
  const [cloudTags, setCloudTags] = useState([]);
  const [collections, setCollections] = useState([]);
  const [collectionName, setCollectionName] = useState('');
  const [savedFilters, setSavedFilters] = useState(null);
  const [feed, setFeed] = useState([]);
  const [series, setSeries] = useState([]);
  const [showFilter, setShowFilter] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const loadedRef = useRef(false);

  useEffect(() => {
    api('/libraries').then((d) => setLibraries(d.items)).catch(() => {});
    api('/users/me/continue').then((d) => setContinueWatching(d.items)).catch(() => {});
    api('/tags').then((d) => setCloudTags(d.items || [])).catch(() => setCloudTags([]));
    api('/collections').then((d) => setCollections(d.items || [])).catch(() => setCollections([]));
    api('/users/me/filters').then((d) => setSavedFilters(d.filters || null)).catch(() => {});
    api('/feed').then((d) => setFeed(d.items || [])).catch(() => setFeed([]));
  }, []);

  useEffect(() => {
    const params = new URLSearchParams();
    if (libraryId) params.set('library_id', libraryId);
    api(`/series?${params}`).then((d) => setSeries(d.items)).catch(() => {});
  }, [libraryId]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(PAGE_SIZE),
      sort,
    });
    if (search) params.set('q', search);
    if (libraryId) params.set('library_id', libraryId);
    if (vtype) params.set('type', vtype);
    if (tag) params.set('tag', tag);
    api(`/videos?${params}`)
      .then((d) => {
        if (cancelled) return;
        if (page === 1) {
          setVideos(d.items);
        } else {
          setVideos((prev) => [...prev, ...d.items]);
        }
        setTotal(d.total);
        setError('');
      })
      .catch((err) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [page, search, libraryId, sort, vtype, tag]);

  function pickTag(name) {
    setTag(name);
    setPage(1);
    if (name) setSearchParams({ tag: name });
    else setSearchParams({});
  }

  function currentFilters() {
    const f = {};
    if (search) f.q = search;
    if (libraryId) f.library_id = libraryId;
    if (tag) f.tag = tag;
    if (vtype) f.type = vtype;
    return f;
  }

  async function saveFilters() {
    try {
      const f = currentFilters();
      const d = await api('/users/me/filters', { method: 'PUT', body: { filters: f } });
      setSavedFilters(d.filters);
      setError('');
    } catch (e) {
      setError(e.message);
    }
  }

  function applyFilters(f) {
    setPage(1);
    setQ(f.q || '');
    setSearch(f.q || '');
    setLibraryId(f.library_id || '');
    setTag(f.tag || '');
    setVtype(f.type || '');
    if (f.tag) setSearchParams({ tag: f.tag });
    else setSearchParams({});
  }

  async function createCollection() {
    const name = collectionName.trim();
    if (!name) return;
    try {
      await api('/collections', { method: 'POST', body: { name, query: currentFilters() } });
      setCollectionName('');
      api('/collections').then((d) => setCollections(d.items || [])).catch(() => {});
    } catch (e) {
      setError(e.message);
    }
  }

  async function deleteCollection(id) {
    try {
      await api(`/collections/${id}`, { method: 'DELETE' });
      setCollections((prev) => prev.filter((c) => c.id !== id));
    } catch (e) {
      setError(e.message);
    }
  }

  function submitSearch(e) {
    e.preventDefault();
    loadedRef.current = false;
    setPage(1);
    setSearch(q.trim());
  }

  function reloadAll() {
    api('/videos?page_size=1').then(() => {
      setPage(1);
      setSearch(search);
    }).catch(() => {});
  }

  const showMore = videos.length < total;

  return (
    <div className="container">
      {feed.length > 0 && (
        <section className="section">
          <h2>{t('browse.recentActivity')}</h2>
          <div className="card feed-box">
            {feed.map((f, i) => (
              <div key={`${f.kind}-${f.created_at}-${i}`} className="feed-row">
                {f.kind === 'comment'
                  ? t('browse.feedComment', { user: f.username, title: f.video_title, text: f.text })
                  : t('browse.feedFavorite', { user: f.username, title: f.video_title })}
                <Link className="section-more" to={`/video/${f.video_id}`}>→</Link>
              </div>
            ))}
          </div>
        </section>
      )}
      {continueWatching.length > 0 && (
        <section className="section">
          <h2>{t('browse.continueWatching')}</h2>
          <div className="card-grid">
            {continueWatching.map((v) => (
              <VideoCard key={v.id} video={v} />
            ))}
          </div>
        </section>
      )}

      {series.length > 0 && (
        <section className="section">
          <div className="section-head">
            <h2>{t('series.title')}</h2>
            <Link to="/series" className="section-more">→</Link>
          </div>
          <div className="card-grid">
            {series.slice(0, 10).map((s) => (
              <SeriesCard key={s.id} series={s} />
            ))}
          </div>
        </section>
      )}

      <section className="section">
        {(collections.length > 0 || savedFilters) && (
          <div className="browse-toolbar browse-save">
            {collections.length > 0 && (
              <span className="browse-group">
                {t('browse.collections')}:
                {collections.map((c) => (
                  <span key={c.id} className="tag-chip">
                    <button className="tag-link" onClick={() => applyFilters(c.query)}>{c.name}</button>
                    <button className="tag-remove" onClick={() => deleteCollection(c.id)} aria-label={t('common.clear')}>×</button>
                  </span>
                ))}
              </span>
            )}
            {savedFilters && (
              <button className="btn small ghost" onClick={() => applyFilters(savedFilters)}>
                {t('browse.savedFilters')}
              </button>
            )}
            <button className="btn small ghost" onClick={saveFilters}>{t('browse.saveFilters')}</button>
            <input
              className="collection-input"
              placeholder={t('browse.collectionName')}
              value={collectionName}
              onChange={(e) => setCollectionName(e.target.value)}
            />
            <button className="btn small ghost" onClick={createCollection}>{t('browse.createCollection')}</button>
          </div>
        )}
        {cloudTags.length > 0 && (
          <div className="tag-cloud">
            {cloudTags.map((t) => (
              <button
                key={t.id}
                className={`tag-chip${tag === t.name ? ' active' : ''}`}
                onClick={() => pickTag(tag === t.name ? '' : t.name)}
              >
                {t.name} <span className="muted">({t.count})</span>
              </button>
            ))}
          </div>
        )}
        {tag && (
          <p className="muted">
            {t('browse.filteredByTag', { tag })}{' '}
            <button className="btn small ghost" onClick={() => pickTag('')}>{t('common.clear')}</button>
          </p>
        )}
        <div className="browse-toolbar">
          <form onSubmit={submitSearch} className="search-form">
            <input
              className="search-input"
              placeholder={t('browse.searchPlaceholder')}
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
            <button className="btn primary">{t('common.search')}</button>
          </form>
          <select value={libraryId} onChange={(e) => { setLibraryId(e.target.value); setPage(1); }}>
            <option value="">{t('browse.allLibraries')}</option>
            {libraries.map((l) => (
              <option key={l.id} value={l.id}>
                {l.name}
              </option>
            ))}
          </select>
          <select value={sort} onChange={(e) => { setSort(e.target.value); setPage(1); }}>
            <option value="title">{t('browse.sortTitle')}</option>
            <option value="year_desc">{t('browse.sortYearDesc')}</option>
            <option value="duration_desc">{t('browse.sortDuration')}</option>
            <option value="added_desc">{t('browse.sortAdded')}</option>
            <option value="favorites_desc">{t('browse.sortFavorites')}</option>
            <option value="fuzzy">{t('browse.sortFuzzy')}</option>
          </select>
          <select value={vtype} onChange={(e) => { setVtype(e.target.value); setPage(1); }}>
            <option value="">{t('browse.typeAll')}</option>
            <option value="movie">{t('browse.typeMovie')}</option>
            <option value="tv">{t('browse.typeTv')}</option>
          </select>
          <button className="btn" onClick={() => setShowFilter(true)}>
            {t('series.pathFilter')}
          </button>
        </div>

        {error && <div className="form-error">{error}</div>}
        {!error && videos.length === 0 && !loading && (
          <div className="empty">{t('browse.empty')}</div>
        )}
        {!error && (
          <div className="card-grid">
            {videos.map((v) => (
              <VideoCard key={v.id} video={v} />
            ))}
          </div>
        )}
        {loading && <div className="loading">{t('common.loading')}</div>}
        {showMore && !loading && (
          <div className="load-more">
            <button className="btn" onClick={() => setPage((p) => p + 1)}>
              {t('browse.loadMore', { loaded: videos.length, total })}
            </button>
          </div>
        )}
      </section>
      {showFilter && (
        <PathFilterModal onClose={() => setShowFilter(false)} onChanged={reloadAll} />
      )}
      <div className="footer-note">
        {t('browse.footerUser', {
          name: user?.display_name || user?.username,
          role: user?.role === 'admin' ? t('browse.roleAdmin') : t('browse.roleUser'),
        })}
      </div>
    </div>
  );
}
