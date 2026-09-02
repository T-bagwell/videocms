package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// metricsRegistry is a small Prometheus-text registry without dependencies.
type metricsRegistry struct {
	started time.Time
	mu      sync.Mutex
	counts  map[string]int64 // key: path|method|status
}

func newMetricsRegistry() *metricsRegistry {
	return &metricsRegistry{started: time.Now(), counts: map[string]int64{}}
}

func (m *metricsRegistry) inc(path, method string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := path + "|" + method + "|" + strconv.Itoa(status)
	m.counts[key]++
}

func (m *metricsRegistry) render(pool *pgxpool.Pool) string {
	m.mu.Lock()
	keys := make([]string, 0, len(m.counts))
	for k := range m.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	b.WriteString("# HELP videocms_http_requests_total HTTP requests by path/method/status.\n")
	b.WriteString("# TYPE videocms_http_requests_total counter\n")
	for _, k := range keys {
		parts := splitN(k, "|", 3)
		fmt.Fprintf(&b, "videocms_http_requests_total{path=%q,method=%q,status=%q} %d\n",
			parts[0], parts[1], parts[2], m.counts[k])
	}
	m.mu.Unlock()

	count := func(sql string) int64 {
		var n int64
		if pool != nil {
			if err := pool.QueryRow(context.Background(), sql).Scan(&n); err == nil {
				return n
			}
		}
		return 0
	}
	b.WriteString("# HELP videocms_videos_total Indexed videos.\n# TYPE videocms_videos_total gauge\n")
	fmt.Fprintf(&b, "videocms_videos_total %d\n", count(`SELECT count(*) FROM videos WHERE available=true`))
	b.WriteString("# HELP videocms_libraries_total Libraries.\n# TYPE videocms_libraries_total gauge\n")
	fmt.Fprintf(&b, "videocms_libraries_total %d\n", count(`SELECT count(*) FROM libraries`))
	b.WriteString("# HELP videocms_users_total Users.\n# TYPE videocms_users_total gauge\n")
	fmt.Fprintf(&b, "videocms_users_total %d\n", count(`SELECT count(*) FROM users`))
	b.WriteString("# HELP videocms_download_jobs_total Download jobs.\n# TYPE videocms_download_jobs_total gauge\n")
	fmt.Fprintf(&b, "videocms_download_jobs_total %d\n", count(`SELECT count(*) FROM downloads`))
	b.WriteString("# HELP videocms_recordings_pending_total Pending recordings.\n# TYPE videocms_recordings_pending_total gauge\n")
	fmt.Fprintf(&b, "videocms_recordings_pending_total %d\n", count(`SELECT count(*) FROM recordings WHERE status='pending'`))
	b.WriteString("# HELP videocms_uptime_seconds Server uptime.\n# TYPE videocms_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "videocms_uptime_seconds %.0f\n", time.Since(m.started).Seconds())
	return b.String()
}

func splitN(s, sep string, n int) []string {
	var out []string
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		out = append(out, s[:idx])
		s = s[idx+len(sep):]
	}
	out = append(out, s)
	return out
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// otelExporter ships minimal OTLP/JSON traces when configured.
type otelExporter struct {
	endpoint string
	ch       chan otelSpan
}

type otelSpan struct {
	traceID    string
	spanID     string
	name       string
	start      time.Time
	end        time.Time
	statusCode int64
	attrs      map[string]string
}

func newOTelExporter(endpoint string) *otelExporter {
	o := &otelExporter{endpoint: endpoint, ch: make(chan otelSpan, 1024)}
	go o.run()
	return o
}

func (o *otelExporter) span(r *http.Request, status int, dur time.Duration) {
	tid := make([]byte, 16)
	sid := make([]byte, 8)
	_, _ = rand.Read(tid)
	_, _ = rand.Read(sid)
	select {
	case o.ch <- otelSpan{
		traceID: hex.EncodeToString(tid), spanID: hex.EncodeToString(sid),
		name:       r.Method + " " + r.URL.Path,
		start:      time.Now().Add(-dur),
		end:        time.Now(),
		statusCode: statusCode(status),
		attrs: map[string]string{
			"http.request.method":       r.Method,
			"url.path":                  r.URL.Path,
			"http.response.status_code": strconv.Itoa(status),
		},
	}:
	default:
	}
}

func statusCode(status int) int64 {
	if status >= 500 {
		return 2
	}
	return 1
}

func (o *otelExporter) run() {
	var batch []otelSpan
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		o.post(batch)
		batch = nil
	}
	for {
		select {
		case s := <-o.ch:
			batch = append(batch, s)
			if len(batch) >= 20 {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

func (o *otelExporter) post(spans []otelSpan) {
	type kv struct {
		Key   string `json:"key"`
		Value struct {
			StringValue string `json:"stringValue"`
		} `json:"value"`
	}
	toKVs := func(m map[string]string) []kv {
		out := make([]kv, 0, len(m))
		for k, v := range m {
			var it kv
			it.Key = k
			it.Value.StringValue = v
			out = append(out, it)
		}
		return out
	}
	type span struct {
		TraceID           string `json:"traceId"`
		SpanID            string `json:"spanId"`
		Name              string `json:"name"`
		Kind              int    `json:"kind"`
		StartTimeUnixNano string `json:"startTimeUnixNano"`
		EndTimeUnixNano   string `json:"endTimeUnixNano"`
		Status            struct {
			Code int64 `json:"code"`
		} `json:"status"`
		Attributes []kv `json:"attributes"`
	}
	payload := struct {
		ResourceSpans []struct {
			Resource struct {
				Attributes []kv `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Scope struct {
					Name string `json:"name"`
				} `json:"scope"`
				Spans []span `json:"spans"`
			} `json:"scopeSpans"`
		} `json:"resourceSpans"`
	}{}
	var ps = &payload.ResourceSpans
	*ps = append(*ps, struct {
		Resource struct {
			Attributes []kv `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Scope struct {
				Name string `json:"name"`
			} `json:"scope"`
			Spans []span `json:"spans"`
		} `json:"scopeSpans"`
	}{})
	(*ps)[0].Resource.Attributes = []kv{{Key: "service.name", Value: struct {
		StringValue string `json:"stringValue"`
	}{StringValue: "videocms"}}}
	(*ps)[0].ScopeSpans = append((*ps)[0].ScopeSpans, struct {
		Scope struct {
			Name string `json:"name"`
		} `json:"scope"`
		Spans []span `json:"spans"`
	}{Scope: struct {
		Name string `json:"name"`
	}{Name: "videocms"}})
	for _, s := range spans {
		var sp span
		sp.TraceID = s.traceID
		sp.SpanID = s.spanID
		sp.Name = s.name
		sp.Kind = 2
		sp.StartTimeUnixNano = strconv.FormatInt(s.start.UnixNano(), 10)
		sp.EndTimeUnixNano = strconv.FormatInt(s.end.UnixNano(), 10)
		sp.Status.Code = s.statusCode
		sp.Attributes = toKVs(s.attrs)
		(*ps)[0].ScopeSpans[0].Spans = append((*ps)[0].ScopeSpans[0].Spans, sp)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint+"/v1/traces", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("otel export failed: %v", err)
		return
	}
	_ = resp.Body.Close()
}

func (a *App) withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		a.metrics.inc(r.URL.Path, r.Method, sw.status)
		if a.tracer != nil {
			a.tracer.span(r, sw.status, time.Since(start))
		}
	})
}

// GET /metrics — Prometheus text exposition.
func (a *App) metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(a.metrics.render(a.pool)))
}
