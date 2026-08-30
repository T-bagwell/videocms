import { useEffect, useState } from 'react';
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../auth.jsx';
import { api, setToken } from '../api.js';
import { SUPPORTED_LANGS, setLang } from '../i18n';

export default function LoginPage() {
  const { login, register } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const from = location.state?.from?.pathname || '/';
  const { t, i18n } = useTranslation();

  const [mode, setMode] = useState('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [sso, setSso] = useState({ oidc: false, saml: false });

  useEffect(() => {
    const sso = searchParams.get('sso_token');
    if (sso) {
      setToken(sso);
      navigate(from, { replace: true });
    }
  }, [searchParams, from, navigate]);

  useEffect(() => {
    api('/auth/sso')
      .then(setSso)
      .catch(() => {});
  }, []);

  async function submit(e) {
    e.preventDefault();
    setError('');
    setBusy(true);
    try {
      if (mode === 'login') {
        await login(username, password);
      } else {
        await register({ username, password, display_name: displayName });
      }
      navigate(from, { replace: true });
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  function ssoLogin() {
    window.location.href = '/api/auth/oidc/start';
  }

  function samlLogin() {
    window.location.href = '/api/auth/saml/login';
  }

  return (
    <div className="login-wrap">
      <form className="login-card" onSubmit={submit}>
        <div className="login-logo">🎬</div>
        <h1>VideoCMS</h1>
        <p className="login-sub">{t('login.subtitle')}</p>

        <select
          className="lang-select login-lang"
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

        {mode === 'register' && (
          <label>
            {t('login.displayName')}
            <input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder={t('login.displayNamePlaceholder')}
            />
          </label>
        )}

        {sso.oidc && (
          <button type="button" className="btn ghost" onClick={ssoLogin}>
            {t('login.ssoOidc')}
          </button>
        )}
        {sso.saml && (
          <button type="button" className="btn ghost" onClick={samlLogin}>
            {t('login.ssoSaml')}
          </button>
        )}
        <label>
          {t('login.username')}
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus />
        </label>
        <label>
          {t('login.password')}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="btn primary" disabled={busy}>
          {busy ? t('login.busy') : mode === 'login' ? t('login.login') : t('login.register')}
        </button>
        <button
          type="button"
          className="link-btn"
          onClick={() => {
            setMode(mode === 'login' ? 'register' : 'login');
            setError('');
          }}
        >
          {mode === 'login' ? t('login.switchToRegister') : t('login.switchToLogin')}
        </button>
        <p className="login-hint">{t('login.hint')}</p>
      </form>
    </div>
  );
}
