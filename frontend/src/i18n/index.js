import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import en from './locales/en.json';
import zh from './locales/zh.json';
import fr from './locales/fr.json';
import ja from './locales/ja.json';
import de from './locales/de.json';

const LANG_KEY = 'videocms_lang';

export const SUPPORTED_LANGS = [
  { code: 'en', label: 'English' },
  { code: 'zh', label: '中文' },
  { code: 'fr', label: 'Français' },
  { code: 'ja', label: '日本語' },
  { code: 'de', label: 'Deutsch' },
];

export function getSavedLang() {
  const saved = localStorage.getItem(LANG_KEY);
  if (saved && SUPPORTED_LANGS.some((l) => l.code === saved)) return saved;
  return 'en';
}

export function setLang(code) {
  localStorage.setItem(LANG_KEY, code);
  i18n.changeLanguage(code);
  document.documentElement.lang = code;
}

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh: { translation: zh },
    fr: { translation: fr },
    ja: { translation: ja },
    de: { translation: de },
  },
  lng: getSavedLang(),
  fallbackLng: 'en',
  interpolation: { escapeValue: false },
});

document.documentElement.lang = i18n.language;

export function fmtDuration(sec) {
  if (!sec || sec <= 0) return i18n.t('common.unknownDuration');
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  if (m >= 60) {
    const h = Math.floor(m / 60);
    return i18n.t('common.hoursMinutes', { h, m: m % 60 });
  }
  return i18n.t('common.minutesSeconds', { m, s: s.toString().padStart(2, '0') });
}

export function fmtBytes(bytes) {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  let v = bytes;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}

export default i18n;

