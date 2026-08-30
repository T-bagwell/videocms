import { afterEach, describe, expect, it } from 'vitest';
import { apiBaseUrl, mediaUrl, publicUrl, setToken } from './api.js';

describe('api url helpers', () => {
  afterEach(() => setToken(null));

  it('mediaUrl appends the JWT token', () => {
    setToken('abc');
    expect(mediaUrl('/videos/1/stream')).toBe('/api/videos/1/stream?token=abc');
  });

  it('mediaUrl keeps existing query parameters', () => {
    setToken('x');
    expect(mediaUrl('/videos/1/hls/playlist.m3u8?start=5')).toBe(
      '/api/videos/1/hls/playlist.m3u8?start=5&token=x',
    );
  });

  it('publicUrl needs no token', () => {
    setToken('abc');
    expect(publicUrl('/share/tok/video/1/stream')).toBe('/api/share/tok/video/1/stream');
  });

  it('apiBaseUrl is same-origin by default', () => {
    expect(apiBaseUrl()).toBe('');
  });
});
