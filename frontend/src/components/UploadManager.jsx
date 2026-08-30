import { forwardRef, useImperativeHandle, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, getToken } from '../api.js';
import { fmtBytes } from '../i18n';

const CHUNK_SIZE = 8 * 1024 * 1024; // keep in sync with the backend default

function parseErrorText(text) {
  try {
    const d = JSON.parse(text);
    if (d?.error) return d.error;
  } catch {
    // non-JSON error body
  }
  return text || 'upload failed';
}

function putChunk(url, blob, token, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', url);
    xhr.setRequestHeader('Authorization', `Bearer ${token}`);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded);
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(parseErrorText(xhr.responseText)));
    };
    xhr.onerror = () => reject(new Error('network error'));
    xhr.send(blob);
  });
}

const UploadManager = forwardRef(function UploadManager({ targetPath }, ref) {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const itemsRef = useRef([]);
  const pausedRef = useRef(new Set());
  const runningRef = useRef(false);

  function update(key, patch) {
    itemsRef.current = itemsRef.current.map((it) =>
      it.key === key ? { ...it, ...patch } : it,
    );
    setItems([...itemsRef.current]);
  }

  async function processItem(item) {
    const total = item.file.size;
    try {
      if (!item.session) {
        const d = await api('/uploads', {
          method: 'POST',
          body: { filename: item.file.name, target_path: targetPath, size: total },
        });
        update(item.key, { session: d });
        item.session = d;
      }
      const cs = item.session.chunk_size || CHUNK_SIZE;
      const received = new Set(item.received || []);
      let index = 0;
      let uploaded = received.size > 0 ? Math.min(received.size * cs, total) : 0;
      update(item.key, { status: 'uploading', uploaded });
      while (index * cs < total) {
        if (pausedRef.current.has(item.key)) {
          update(item.key, { status: 'paused' });
          return;
        }
        if (received.has(index)) {
          index += 1;
          continue;
        }
        const start = index * cs;
        const end = Math.min(start + cs, total);
        await putChunk(
          `/api/uploads/${item.session.id}/chunk/${index}`,
          item.file.slice(start, end),
          getToken(),
          (loaded) => update(item.key, { uploaded: start + loaded }),
        );
        received.add(index);
        uploaded = start + (end - start);
        update(item.key, { received: [...received], uploaded });
        index += 1;
      }
      await api(`/uploads/${item.session.id}/complete`, { method: 'POST' });
      update(item.key, { status: 'done', uploaded: total, error: '' });
    } catch (e) {
      if (pausedRef.current.has(item.key)) {
        update(item.key, { status: 'paused' });
        return;
      }
      update(item.key, { status: 'error', error: e.message || t('uploads.statusError') });
    }
  }

  async function pump() {
    if (runningRef.current) return;
    runningRef.current = true;
    try {
      for (;;) {
        const next = itemsRef.current.find((it) => it.status === 'queued');
        if (!next) break;
        await processItem(next);
      }
    } finally {
      runningRef.current = false;
    }
  }

  useImperativeHandle(ref, () => ({
    addFiles(fileList) {
      const files = Array.from(fileList || []).filter((f) => f.size > 0);
      if (files.length === 0) return;
      const now = Date.now();
      const added = files.map((f, i) => ({
        key: `${f.name}-${f.size}-${now}-${i}-${Math.random()}`,
        file: f,
        session: null,
        received: [],
        status: 'queued',
        error: '',
        uploaded: 0,
      }));
      itemsRef.current = [...itemsRef.current, ...added];
      setItems([...itemsRef.current]);
      void pump();
    },
  }));

  function pause(key) {
    pausedRef.current.add(key);
    update(key, { status: 'paused' });
  }

  function resume(key) {
    pausedRef.current.delete(key);
    update(key, { status: 'queued', error: '' });
    void pump();
  }

  function retry(key) {
    update(key, { status: 'queued', error: '' });
    void pump();
  }

  async function cancel(key) {
    const item = itemsRef.current.find((it) => it.key === key);
    if (item?.session) {
      try {
        await api(`/uploads/${item.session.id}`, { method: 'DELETE' });
      } catch {
        // session may already be gone; drop it from the queue anyway
      }
    }
    pausedRef.current.delete(key);
    itemsRef.current = itemsRef.current.filter((it) => it.key !== key);
    setItems([...itemsRef.current]);
  }

  if (items.length === 0) {
    return <div className="empty">{t('uploads.empty')}</div>;
  }

  return (
    <div className="playlist-list">
      {items.map((it) => {
        const pct = it.file.size > 0 ? Math.round((it.uploaded / it.file.size) * 100) : 0;
        return (
          <div key={it.key} className="card playlist-card">
            <div className="playlist-main">
              <div className="playlist-icon">⬆</div>
              <div style={{ flex: 1 }}>
                <div className="playlist-name">
                  {it.file.name}
                  <span className={`status-badge status-${it.status}`}>
                    {t(`uploads.status${it.status[0].toUpperCase()}${it.status.slice(1)}`)}
                  </span>
                </div>
                <div className="muted">
                  {fmtBytes(it.uploaded)} / {fmtBytes(it.file.size)} ({pct}%)
                </div>
                {it.error && <div className="form-error small">{it.error}</div>}
              </div>
            </div>
            <div className="detail-actions">
              {it.status === 'uploading' && (
                <button className="btn ghost" onClick={() => pause(it.key)}>
                  {t('uploads.pause')}
                </button>
              )}
              {it.status === 'paused' && (
                <button className="btn ghost" onClick={() => resume(it.key)}>
                  {t('uploads.resume')}
                </button>
              )}
              {it.status === 'error' && (
                <button className="btn ghost" onClick={() => retry(it.key)}>
                  {t('uploads.retry')}
                </button>
              )}
              {it.status !== 'done' && (
                <button className="btn ghost" onClick={() => cancel(it.key)}>
                  {t('uploads.cancel')}
                </button>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
});

export default UploadManager;
