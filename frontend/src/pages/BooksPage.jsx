import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { api } from '../api.js';
import { fmtBytes } from '../i18n';

export default function BooksPage() {
  const { t } = useTranslation();
  const [books, setBooks] = useState([]);
  const [err, setErr] = useState('');

  useEffect(() => {
    api('/books')
      .then((d) => setBooks(d.items || []))
      .catch((e) => setErr(e.message));
  }, []);

  return (
    <div className="container">
      <h1>{t('nav.books')}</h1>
      {err && <div className="form-error">{err}</div>}
      {books.length === 0 && !err && <div className="empty">{t('books.noBooks')}</div>}
      <div className="video-grid">
        {books.map((b) => (
          <Link key={b.id} to={`/books/${b.id}`} className="poster-link">
            <div className="album-card">
              <div className="book-cover">{b.format === 'cbz' ? '🗯' : '📖'}</div>
              <div className="album-name">{b.title}</div>
              <div className="muted small">
                {b.format.toUpperCase()} · {fmtBytes(b.size_bytes)}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
