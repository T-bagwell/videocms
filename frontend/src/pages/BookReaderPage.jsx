import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api, mediaUrl } from '../api.js';

function encodePath(path) {
  return String(path || '')
    .split('/')
    .map((seg) => encodeURIComponent(seg))
    .join('/');
}

export default function BookReaderPage() {
  const { id } = useParams();
  const { t } = useTranslation();
  const [book, setBook] = useState(null);
  const [pages, setPages] = useState([]);
  const [page, setPage] = useState(0);
  const [spine, setSpine] = useState([]);
  const [chapter, setChapter] = useState(0);
  const [err, setErr] = useState('');

  useEffect(() => {
    api(`/books/${id}`).then(setBook).catch((e) => setErr(e.message));
    api(`/books/${id}/pages`)
      .then((d) => {
        setPages(d.pages || []);
        setPage(0);
      })
      .catch(() => setPages([]));
    api(`/books/${id}/epub/spine`)
      .then((d) => {
        setSpine(d.chapters || []);
        setChapter(0);
      })
      .catch(() => setSpine([]));
  }, [id]);

  useEffect(() => {
    function onKey(e) {
      const tag = (e.target.tagName || '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') return;
      if (book?.format === 'cbz' && pages.length > 0) {
        if (e.key === 'ArrowRight' || e.key === ' ') {
          e.preventDefault();
          setPage((p) => Math.min(pages.length - 1, p + 1));
        } else if (e.key === 'ArrowLeft') {
          e.preventDefault();
          setPage((p) => Math.max(0, p - 1));
        }
      } else if (book?.format === 'epub' && spine.length > 0) {
        if (e.key === 'ArrowRight' || e.key === ' ') {
          e.preventDefault();
          setChapter((c) => Math.min(spine.length - 1, c + 1));
        } else if (e.key === 'ArrowLeft') {
          e.preventDefault();
          setChapter((c) => Math.max(0, c - 1));
        }
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [book, pages, spine]);

  if (err && !book) return <div className="container"><div className="form-error">{err}</div></div>;
  if (!book) return <div className="container"><div className="loading">{t('common.loading')}</div></div>;

  const isCbz = book.format === 'cbz' && pages.length > 0;
  const isEpub = book.format === 'epub' && spine.length > 0;
  const isPdf = book.format === 'pdf';

  return (
    <div className="container">
      <div className="reader-head">
        <h1>{book.title}</h1>
        <Link className="btn small ghost" to="/books">← {t('nav.books')}</Link>
      </div>
      <div className="card reader-shell">
        {isCbz && (
          <>
            <div className="cbz-stage">
              <img
                className="cbz-page"
                src={mediaUrl(`/books/${id}/pages/${page}`)}
                alt={`${t('books.page')} ${page + 1}`}
              />
            </div>
            <div className="reader-tools">
              <button className="btn small" onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={page === 0}>
                {t('books.prev')}
              </button>
              <span className="muted small">{t('books.pageOf', { current: page + 1, total: pages.length })}</span>
              <button
                className="btn small"
                onClick={() => setPage((p) => Math.min(pages.length - 1, p + 1))}
                disabled={page >= pages.length - 1}
              >
                {t('books.next')}
              </button>
            </div>
          </>
        )}
        {isEpub && (
          <>
            <iframe
              className="reader-frame"
              title={book.title}
              src={mediaUrl(`/books/${id}/epub/resource/${encodePath(spine[chapter].path)}`)}
            />
            <div className="reader-tools">
              <button className="btn small" onClick={() => setChapter((c) => Math.max(0, c - 1))} disabled={chapter === 0}>
                {t('books.prev')}
              </button>
              <span className="muted small">{t('books.chapterOf', { current: chapter + 1, total: spine.length })}</span>
              <button
                className="btn small"
                onClick={() => setChapter((c) => Math.min(spine.length - 1, c + 1))}
                disabled={chapter >= spine.length - 1}
              >
                {t('books.next')}
              </button>
            </div>
          </>
        )}
        {isPdf && (
          <iframe className="reader-frame" title={book.title} src={mediaUrl(`/books/${id}/file`)} />
        )}
        {!isCbz && !isEpub && !isPdf && <div className="empty">{t('books.unreadable')}</div>}
      </div>
    </div>
  );
}
