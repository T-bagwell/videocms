import { Link, NavLink, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../auth.jsx';
import { SUPPORTED_LANGS, setLang } from '../i18n';

export default function Navbar() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();

  return (
    <header className="navbar">
      <Link to="/" className="brand">
        🎬 VideoCMS
      </Link>
      <nav className="nav-links">
        <NavLink to="/" end className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.home')}
        </NavLink>
        <NavLink to="/playlists" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.playlists')}
        </NavLink>
        <NavLink to="/favorites" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.favorites')}
        </NavLink>
        {user?.role === 'admin' && (
          <NavLink to="/admin" className={({ isActive }) => (isActive ? 'active' : '')}>
            {t('nav.admin')}
          </NavLink>
        )}
      </nav>
      <div className="nav-user">
        <select
          className="lang-select"
          value={i18n.language}
          onChange={(e) => setLang(e.target.value)}
          aria-label={t('nav.language')}
        >
          {SUPPORTED_LANGS.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
        <span className="user-name">{user?.display_name || user?.username}</span>
        <button
          className="btn ghost"
          onClick={() => {
            logout();
            navigate('/login');
          }}
        >
          {t('nav.logout')}
        </button>
      </div>
    </header>
  );
}
