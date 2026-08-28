import { describe, expect, it } from 'vitest';
import en from './locales/en.json';
import zh from './locales/zh.json';
import fr from './locales/fr.json';
import ja from './locales/ja.json';
import de from './locales/de.json';
import { SUPPORTED_LANGS, fmtBytes, fmtDuration } from './index.js';

const LOCALES = { en, zh, fr, ja, de };

function flattenKeys(obj, prefix = '') {
  return Object.entries(obj).flatMap(([key, value]) => {
    const full = prefix ? `${prefix}.${key}` : key;
    return value && typeof value === 'object' ? flattenKeys(value, full) : [full];
  });
}

describe('i18n locales', () => {
  const enKeys = flattenKeys(en).sort();

  it('every locale has the same key structure as English', () => {
    for (const [code, dict] of Object.entries(LOCALES)) {
      expect(flattenKeys(dict).sort(), code).toEqual(enKeys);
    }
  });

  it('has no empty translation strings', () => {
    const walk = (code, obj) => {
      for (const value of Object.values(obj)) {
        if (value && typeof value === 'object') {
          walk(code, value);
        } else {
          expect(String(value).trim(), code).not.toBe('');
        }
      }
    };
    for (const [code, dict] of Object.entries(LOCALES)) {
      walk(code, dict);
    }
  });

  it('registers every locale in SUPPORTED_LANGS', () => {
    expect(SUPPORTED_LANGS.map((l) => l.code).sort()).toEqual(Object.keys(LOCALES).sort());
  });
});

describe('fmtDuration', () => {
  it('formats hours and minutes', () => {
    expect(fmtDuration(3661)).toBe(en.common.hoursMinutes.replace('{{h}}', '1').replace('{{m}}', '1'));
  });

  it('formats minutes and seconds with padding', () => {
    expect(fmtDuration(125)).toBe(en.common.minutesSeconds.replace('{{m}}', '2').replace('{{s}}', '05'));
  });

  it('returns the unknown label for missing durations', () => {
    expect(fmtDuration(0)).toBe(en.common.unknownDuration);
    expect(fmtDuration(null)).toBe(en.common.unknownDuration);
  });
});

describe('fmtBytes', () => {
  it('formats bytes, KB, MB and GB', () => {
    expect(fmtBytes(0)).toBe('0 B');
    expect(fmtBytes(512)).toBe('512.0 B');
    expect(fmtBytes(2048)).toBe('2.0 KB');
    expect(fmtBytes(5 * 1024 * 1024)).toBe('5.0 MB');
    expect(fmtBytes(3 * 1024 ** 3)).toBe('3.0 GB');
  });
});
