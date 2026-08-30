package api

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"videocms/backend/internal/media"
)

const dlnaDidlNS = `xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"`

// dlnaGuard rejects requests from non-allowed clients before any response.
func (a *App) dlnaGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.dlna == nil || !a.dlna.Allowed(r.RemoteAddr) {
			writeErr(w, http.StatusForbidden, "dlna not enabled or source not allowed")
			return
		}
		next(w, r)
	}
}

// GET /dlna/device.xml
func (a *App) dlnaDeviceDescription(w http.ResponseWriter, r *http.Request) {
	udn := "uuid:" + a.dlna.UDN()
	body := fmt.Sprintf(`<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <device>
    <deviceType>%s</deviceType>
    <friendlyName>%s</friendlyName>
    <manufacturer>VideoCMS</manufacturer>
    <manufacturerURL>https://github.com/T-bagwell/videocms</manufacturerURL>
    <modelName>VideoCMS</modelName>
    <modelNumber>1.0</modelNumber>
    <UDN>%s</UDN>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ContentDirectory:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ContentDirectory</serviceId>
        <SCPDURL>/dlna/scpd.xml</SCPDURL>
        <controlURL>/dlna/control/ContentDirectory</controlURL>
        <eventSubURL>/dlna/event/ContentDirectory</eventSubURL>
      </service>
    </serviceList>
  </device>
</root>`, media.DLNADeviceType, xmlEscape(a.dlna.FriendlyName()), udn)
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write([]byte(body))
}

// GET /dlna/scpd.xml — minimal ContentDirectory service description.
func (a *App) dlnaSCPD(w http.ResponseWriter, r *http.Request) {
	body := `<?xml version="1.0"?>
<scpd xmlns="urn:schemas-upnp-org:service-1-0">
  <specVersion><major>1</major><minor>0</minor></specVersion>
  <actionList>
    <action>
      <name>Browse</name>
      <argumentList>
        <argument><name>ObjectID</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_ObjectID</relatedStateVariable></argument>
        <argument><name>BrowseFlag</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_BrowseFlag</relatedStateVariable></argument>
        <argument><name>Filter</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Filter</relatedStateVariable></argument>
        <argument><name>StartingIndex</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Index</relatedStateVariable></argument>
        <argument><name>RequestedCount</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>SortCriteria</name><direction>in</direction><relatedStateVariable>A_ARG_TYPE_SortCriteria</relatedStateVariable></argument>
        <argument><name>Result</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Result</relatedStateVariable></argument>
        <argument><name>NumberReturned</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>TotalMatches</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_Count</relatedStateVariable></argument>
        <argument><name>UpdateID</name><direction>out</direction><relatedStateVariable>A_ARG_TYPE_UpdateID</relatedStateVariable></argument>
      </argumentList>
    </action>
  </actionList>
  <serviceStateTable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_ObjectID</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_BrowseFlag</name><dataType>string</dataType><allowedValueList><allowedValue>BrowseMetadata</allowedValue><allowedValue>BrowseDirectChildren</allowedValue></allowedValueList></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Filter</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Index</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Count</name><dataType>ui4</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_SortCriteria</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_Result</name><dataType>string</dataType></stateVariable>
    <stateVariable sendEvents="no"><name>A_ARG_TYPE_UpdateID</name><dataType>ui4</dataType></stateVariable>
  </serviceStateTable>
</scpd>`
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write([]byte(body))
}

// GET /dlna/content/{id} — simple GET-based browse used by lightweight clients.
func (a *App) dlnaBrowseGET(w http.ResponseWriter, r *http.Request) {
	objectID := r.PathValue("id")
	if objectID == "0" || objectID == "" {
		objectID = "0"
	}
	didl, _, err := a.dlnaBrowse(r.Context(), objectID, a.dlna.BaseURL(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "dlna browse failed")
		return
	}
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><DIDL-Lite %s>%s</DIDL-Lite>`, dlnaDidlNS, didl)
}

// POST /dlna/control/ContentDirectory — SOAP Browse used by DLNA players.
func (a *App) dlnaContentControl(w http.ResponseWriter, r *http.Request) {
	objectID, err := parseSOAPBrowseObjectID(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid SOAP request")
		return
	}
	didl, total, err := a.dlnaBrowse(r.Context(), objectID, a.dlna.BaseURL(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "dlna browse failed")
		return
	}
	body := fmt.Sprintf(
		`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1"><Result>&lt;DIDL-Lite %s&gt;%s&lt;/DIDL-Lite&gt;</Result><NumberReturned>%d</NumberReturned><TotalMatches>%d</TotalMatches><UpdateID>0</UpdateID></u:BrowseResponse></s:Body></s:Envelope>`,
		dlnaDidlNS, xmlEscape(didl), total, total)
	w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
	_, _ = w.Write([]byte(body))
}

// dlnaBrowse lists libraries at the root or the videos of a library.
func (a *App) dlnaBrowse(ctx context.Context, objectID, base string) (string, int, error) {
	if objectID == "0" {
		rows, err := a.pool.Query(ctx,
			`SELECT id, name, video_count FROM libraries WHERE blocked=false ORDER BY name`)
		if err != nil {
			return "", 0, err
		}
		defer rows.Close()
		var b strings.Builder
		count := 0
		for rows.Next() {
			var id uuid.UUID
			var name string
			var videos int
			if err := rows.Scan(&id, &name, &videos); err != nil {
				return "", 0, err
			}
			fmt.Fprintf(&b, `<container id="lib-%s" parentID="0" restricted="1"><dc:title>%s</dc:title><upnp:class>object.container.storageFolder</upnp:class><upnp:storageUsed>%d</upnp:storageUsed></container>`,
				id, xmlEscape(name), videos)
			count++
		}
		return b.String(), count, rows.Err()
	}

	libID, ok := strings.CutPrefix(objectID, "lib-")
	if !ok {
		return "", 0, fmt.Errorf("unknown object id %q", objectID)
	}
	id, err := uuid.Parse(libID)
	if err != nil {
		return "", 0, fmt.Errorf("bad library id")
	}
	rows, err := a.pool.Query(ctx, `
		SELECT v.id, v.title, v.year, v.duration_sec, v.width, v.height, v.size_bytes,
		       v.poster_path <> '' AS has_poster, v.filename, v.file_path, v.container
		FROM videos v WHERE v.library_id=$1 AND v.available=true
		ORDER BY v.title`, id)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()
	var b strings.Builder
	count := 0
	for rows.Next() {
		var vid uuid.UUID
		var title string
		var year int
		var dur float64
		var width, height int
		var size int64
		var hasPoster bool
		var filename, filePath, container string
		if err := rows.Scan(&vid, &title, &year, &dur, &width, &height, &size,
			&hasPoster, &filename, &filePath, &container); err != nil {
			return "", 0, err
		}
		mime := media.ContentTypeFor(filePath)
		res := fmt.Sprintf(`<res protocolInfo="http-get:*:%s:DLNA.ORG_OP=01;DLNA.ORG_FLAGS=01700000000000000000000000000000" size="%d" duration="%s"%s>%s/dlna/video/%s/stream</res>`,
			mime, size, dlnaDuration(dur), dlnaResolution(width, height), base, vid)
		date := ""
		if year > 0 {
			date = fmt.Sprintf(`<dc:date>%d</dc:date>`, year)
		}
		poster := ""
		if hasPoster {
			poster = fmt.Sprintf(`<upnp:albumArtURI>%s/dlna/video/%s/poster</upnp:albumArtURI>`, base, vid)
		}
		fmt.Fprintf(&b, `<item id="vid-%s" parentID="lib-%s" restricted="1"><dc:title>%s</dc:title><dc:creator>VideoCMS</dc:creator><upnp:class>object.item.videoItem</upnp:class>%s%s%s</item>`,
			vid, id, xmlEscape(title), date, poster, res)
		count++
	}
	return b.String(), count, rows.Err()
}

// GET /dlna/video/{id}/stream — direct file streaming without authentication.
func (a *App) dlnaVideoStream(w http.ResponseWriter, r *http.Request) {
	path, ok := a.dlnaVideoPath(r)
	if !ok {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	media.ServeVideoFile(w, r, path, media.ContentTypeFor(path), "inline")
}

// GET /dlna/video/{id}/poster
func (a *App) dlnaVideoPoster(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "video not found")
		return
	}
	var poster string
	var has bool
	if err := a.pool.QueryRow(r.Context(),
		`SELECT poster_path, has_poster FROM videos WHERE id=$1`, id).Scan(&poster, &has); err != nil || !has {
		writeErr(w, http.StatusNotFound, "poster not found")
		return
	}
	http.ServeFile(w, r, poster)
}

func (a *App) dlnaVideoPath(r *http.Request) (string, bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return "", false
	}
	var path string
	if err := a.pool.QueryRow(r.Context(),
		`SELECT file_path FROM videos WHERE id=$1 AND available=true`, id).Scan(&path); err != nil {
		return "", false
	}
	return path, true
}

func parseSOAPBrowseObjectID(r *http.Request) (string, error) {
	defer func() { _ = r.Body.Close() }()
	var env struct {
		Body struct {
			Browse struct {
				ObjectID string `xml:"ObjectID"`
			} `xml:"Browse"`
		} `xml:"Body"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&env); err != nil {
		return "", err
	}
	if env.Body.Browse.ObjectID == "" {
		return "", fmt.Errorf("missing ObjectID")
	}
	return env.Body.Browse.ObjectID, nil
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func dlnaDuration(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

func dlnaResolution(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	return fmt.Sprintf(` resolution="%dx%d"`, w, h)
}
