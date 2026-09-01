import { useState } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../auth.jsx';
import { api } from '../api.js';
import { SUPPORTED_LANGS, setLang } from '../i18n';

export default function Navbar() {
  const [parentalOpen, setParentalOpen] = useState(false);
  const [pin, setPin] = useState('');
  const [newPin, setNewPin] = useState('');
  const [pinMsg, setPinMsg] = useState('');
  const [pinErr, setPinErr] = useState('');
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();

  async function setMyPin() {
    setPinErr('');
    setPinMsg('');
    try {
      await api('/users/me/pin', { method: 'PUT', body: { pin: newPin.trim() } });
      setPinMsg(t('nav.parentalPinSaved'));
      setNewPin('');
    } catch (e) {
      setPinErr(e.message);
    }
  }

  async function unlock() {
    setPinErr('');
    setPinMsg('');
    try {
      const d = await api('/users/me/pin/verify', { method: 'POST', body: { pin } });
      localStorage.setItem('videocms_unlock', d.unlock_token);
      setTimeout(() => localStorage.removeItem('videocms_unlock'), (d.expires_in || 300) * 1000);
      setPin('');
      setPinMsg(t('nav.parentalUnlocked'));
    } catch (e) {
      setPinErr(e.message);
    }
  }

  return (
    <header className="navbar">
      <Link to="/" className="brand">
        🎬 VideoCMS
      </Link>
      <nav className="nav-links">
        <NavLink to="/" end className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.home')}
        </NavLink>
        <NavLink to="/series" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.tv')}
        </NavLink>
        <NavLink to="/playlists" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.playlists')}
        </NavLink>
        <NavLink to="/favorites" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.favorites')}
        </NavLink>
        <NavLink to="/music" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.music')}
        </NavLink>
        <NavLink to="/books" className={({ isActive }) => (isActive ? 'active' : '')}>
          {t('nav.books')}
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
        <button className="btn ghost" onClick={() => setParentalOpen(true)}>
          {t('nav.parentalLock')}
        </button>
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
      {parentalOpen && (
        <div className="modal-backdrop" onClick={() => setParentalOpen(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h3>{t('nav.parentalLock')}</h3>
            {pinMsg && <div className="toast toast-success">{pinMsg}</div>}
            {pinErr && <div className="form-error">{pinErr}</div>}
            <label>
              {t('nav.parentalUnlockPin')}
              <input type="password" value={pin} onChange={(e) => setPin(e.target.value)} />
            </label>
            <button className="btn primary" onClick={unlock}>{t('nav.parentalUnlock')}</button>
            <label>
              {t('nav.parentalSetPin')}
              <input type="password" value={newPin} onChange={(e) => setNewPin(e.target.value)} />
            </label>
            <button className="btn ghost" onClick={setMyPin}>{t('nav.parentalSavePin')}</button>
            <div className="modal-actions">
              <button className="btn ghost" onClick={() => setParentalOpen(false)}>{t('common.close')}</button>
            </div>
          </div>
        </div>
      )}
    </header>
  );
}
