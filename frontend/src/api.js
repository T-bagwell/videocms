const TOKEN_KEY = 'videocms_token';

// Resolve the API base URL: a runtime override set by the deployment (before
// the app loads) wins, then the build-time VITE_API_BASE_URL, then same-origin.
function resolveApiBase() {
  if (typeof window !== 'undefined' && window.__VIDEOCMS_API_BASE__) {
    return String(window.__VIDEOCMS_API_BASE__).replace(/\/+$/, '');
  }
  const built = import.meta.env?.VITE_API_BASE_URL;
  if (built) return String(built).replace(/\/+$/, '');
  return '';
}

const API_BASE = resolveApiBase();

export function apiBaseUrl() {
  return API_BASE;
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token);
  else localStorage.removeItem(TOKEN_KEY);
}

export function mediaUrl(path) {
  const token = getToken();
  return `${API_BASE}/api${path}${path.includes('?') ? '&' : '?'}token=${encodeURIComponent(token || '')}`;
}

// publicUrl builds an API URL that needs no auth (public share endpoints).
export function publicUrl(path) {
  return `${API_BASE}/api${path}`;
}

export async function api(path, { method = 'GET', body, form } = {}) {
  const headers = {};
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  if (body && !form) headers['Content-Type'] = 'application/json';

  const res = await fetch(`${API_BASE}/api${path}`, {
    method,
    headers,
    body: form || (body ? JSON.stringify(body) : undefined),
  });

  let data = {};
  try {
    data = await res.json();
  } catch {
    // non-JSON response
  }
  if (res.status === 401) {
    setToken(null);
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login';
    }
  }
  if (!res.ok) {
    throw new Error(data.error || `Request failed (${res.status})`);
  }
  return data;
}
