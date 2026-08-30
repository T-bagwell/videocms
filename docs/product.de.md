# VideoCMS — Produktdokumentation

> **Sprachen:** [English](product.md) | [中文](product.zh-CN.md) | [Français](product.fr.md) | [日本語](product.ja.md) | Deutsch

## 1. Was ist VideoCMS?

VideoCMS ist ein selbst gehostetes Videoverwaltungssystem. Zeigen Sie auf Ordner auf
Ihrer Server-Festplatte, scannen Sie einmal, und alle Videos werden zu einer
durchsuchbaren Mediathek — mit Postern, Metadaten, Wiedergabefortschritt, Favoriten,
Wiedergabelisten und einer Kategorie „Serien“ für nummerierte Folgen.

Es basiert auf **Go + React + PostgreSQL** und läuft vollständig auf Ihrer eigenen
Hardware. Ihre Videos verlassen Ihr Netzwerk nie, außer Sie teilen sie bewusst.

## 2. Funktionen

| Bereich | Was Sie erhalten |
| --- | --- |
| Mediatheken | Beliebige Server-Ordner; per Pfad oder mit integrierter Ordnerauswahl |
| Scan | Automatische Erkennung von mp4/mkv/webm/avi/mov/ts… mit Live-Fortschritt und Abbruch |
| Metadaten | Codec/Auflösung/Dauer via ffprobe, automatisch generierte Poster, editierbarer Titel/Jahr/Beschreibung/Genres |
| TMDB-Scraping | Optionale Online-Anreicherung: lokalisierte Titel, Beschreibungen, Genres und Poster |
| Wiedergabe | Native Wiedergabe von H.264/WebM im Browser; automatisches HLS-Transkodieren für MKV/HEVC |
| Serien | Nummerierte Dateien (S01E01, EP1, 第1集…) automatisch nach Folge sortiert gruppiert |
| Persönlich | Weiterschauen, Favoriten, Wiedergabelisten mit fortlaufender Wiedergabe |
| Benutzer | Registrierung/Login, Admin-Rollen, Benutzerverwaltung |
| Inhalts-Blockierung | Admins blockieren Medien per Titel — für alle ausgeblendet, Dateien und Einträge bleiben erhalten, jederzeit freigebbar |
| Oberfläche | 5 Sprachen: Englisch (Standard), 中文, Français, 日本語, Deutsch |

## Screenshots

> *In Kürze — starten Sie `make serve` und öffnen Sie `http://<Server-IP>:8080`,
> um die Oberfläche zu sehen.*

## 3. Schnellstart

### Voraussetzungen

- Ein Server (macOS/Linux/Windows) mit Go 1.26+ zum Kompilieren (oder vorkompiliertes Binary)
- PostgreSQL 14+
- ffmpeg/ffprobe (für Metadaten, Poster und Transkodierung)
- Node.js 20+ (nur für Frontend-Entwicklung; die UI wird vom Backend ausgeliefert)

### Installation

```bash
createdb videocms                       # oder: docker compose up -d db
cd backend && go run ./cmd/server       # erster Start: Tabellen + admin/admin123
cd frontend && npm install && npm run dev   # http://localhost:5173
```

Für LAN-/Handy-Zugriff einmal bauen und einen einzigen Port verwenden:

```bash
make serve                              # http://<LAN-IP>:8080
```

### Erste Anmeldung

Öffnen Sie die Weboberfläche und melden Sie sich mit dem ersten Administrator
**admin / admin123** an (Passwort sofort ändern: Verwaltung → Benutzer → Passwort zurücksetzen).

### Erste Mediathek hinzufügen

1. Gehen Sie zu **Verwaltung → Bibliotheken**
2. Name eingeben, dann einen **absoluten** Server-Pfad (z. B. `/media/movies`)
   tippen oder **Durchsuchen…** verwenden — relative Pfade werden abgelehnt
3. **Scannen** klicken — der Zähler steigt live; Videos erscheinen auf der Startseite

## 4. Benutzerhandbuch

### 4.1 Stöbern und suchen

- Die Startseite zeigt Weiterschauen, Serien und das Video-Raster
- Die Suche trifft Titel, Beschreibung und Dateinamen
- Filter nach Bibliothek und Typ (Alle / Filme / Serien), Sortierung nach Titel,
  Jahr, Dauer, Hinzugefügt oder Beliebtheit
- „Mehr laden“ paginiert das Raster

### 4.2 Videos abspielen

- Klicken Sie auf ein Video für die Detailseite (Poster, Metadaten, Beschreibung)
- **Abspielen** startet; vorhandener Fortschritt wird automatisch fortgesetzt
- H.264 MP4/WebM laufen nativ; MKV/HEVC werden live transkodiert
  (erste Wiedergabe dauert einige Sekunden; „Transkodiert abspielen“ als Fallback)
- Bei Videos mit mehreren Audiospuren kann während der HLS-Wiedergabe die Spur
  gewechselt werden (Audioselektor im Player)
- Untertitel lassen sich bei der Direktwiedergabe um ±0,5 s verschieben;
  der Offset wird pro Benutzer und Video gespeichert
- Beim Überfahren der Fortschrittsleiste erscheinen Vorschaubilder; ein Klick
  springt an die gewünschte Position
- **Download** liefert einen MKV/MP4-Remux mit gewählter Audiospur und
  Untertiteln (ohne Re-Encoding) oder die Originaldatei
- Fortschritt wird alle 5 Sekunden sowie bei Pause/Ende gespeichert

### 4.3 Favoriten und Wiedergabelisten

- ☆ Favorit auf der Detailseite; verwalten unter „Meine Favoriten“
- Wiedergabelisten über die Seite Wiedergabelisten oder direkt aus einem Video
  („+ Zur Wiedergabeliste“)
- Eine Liste kann in Reihenfolge abgespielt werden (▶ Alle abspielen) mit sichtbarer Warteschlange

### 4.4 Serien

Videos mit Folgenmarkern im Dateinamen werden nach einem Scan automatisch gruppiert.
Unterstützte Muster: `S01E01`, `E01`, `EP1`, `第1集`, Zahlen am Ende (`1 (4)`, `-535`).
Eine Gruppe braucht mindestens 2 Folgen, um eine Serie zu werden.

- Die Seite **Serien** listet alle Serien mit Postern und Folgenanzahl
- Eine Serienseite zeigt die Folgen sortiert nach Staffel → Folge
- Serien werden bei jedem Scan neu aufgebaut; Serien mit weniger als 2 verfügbaren
  Folgen werden automatisch bereinigt

### 4.5 Tipps zur Wiedergabe

- Der Player nutzt die nativen Browser-Steuerungen — **Leertaste** zum Abspielen/Pausieren,
  **F** für Vollbild, **←/→** zum Suchen, **↑/↓** für Lautstärke, **M** für Stummschaltung
- Der Fortschritt wird automatisch fortgesetzt; „Weiterschauen“ bringt Sie an die letzte Position
- MKV/HEVC werden live nach HLS transkodiert — die erste Wiedergabe dauert einige
  Sekunden, danach funktionieren Suchen und automatisches Abspielen der nächsten Folge normal
- Die transkodierte Wiedergabe nutzt eine mehrstufige adaptive Qualitätsleiter —
  über den Qualitätsauswahl oberhalb des Players umschaltbar (Standard: Auto)
- Untertitel (danebenliegend, eingebettet oder hochgeladen) lassen sich im Player
  ein- und ausschalten, zwischen Sprachen wechseln, wenn mehrere Spuren existieren,
  und jeder Benutzer kann pro Video seinen eigenen Standard setzen (Admins den
  globalen Standard)
- Auf dem Handy lohnt sich das Querformat; nutzen Sie die Warteschlange unter dem
  Player zum Wechseln zwischen Folgen

### 4.6 Teilen

- Öffnen Sie ein Video, eine Serie oder eine Wiedergabeliste und klicken Sie auf
  **Teilen**, um einen Link zu erstellen (Standard 7 Tage, von 1 Stunde bis 1 Jahr)
- Jeder mit dem Link kann auf einer öffentlichen Seite ansehen — kein Konto
  erforderlich; Serien- und Wiedergabelisten-Links spielen die gesamte Warteschlange
- Links laufen automatisch ab und können jederzeit im selben Dialog widerrufen werden
- Links können passwortgeschützt werden — auf der öffentlichen Seite wird vor dem
  Laden der Inhalt das Passwort abgefragt
- Links können auf eine Liste erlaubter Domains beschränkt werden — Anfragen
  von anderen Hosts werden abgelehnt
- Geteilte Inhalte respektieren die Verwaltungskontrollen: blockierte Titel und
  Bibliotheken erscheinen nie in Teilen-Links

## 5. Administrationshandbuch

### 5.1 Übersicht

Statistiken: Videos, Bibliotheken, Benutzer, Wiedergabelisten, Favoriten, Serien und belegter Speicher.

- **Backup exportieren / importieren**: vollständiges JSON-Backup der
  Server-Metadaten herunterladen oder wiederherstellen (Bibliotheken und Videos
  werden nach Pfad idempotent zusammengeführt; persönliche Daten werden nur für
  bestehende Benutzer wiederhergestellt)

### 5.2 Bibliotheken

- Hinzufügen mit Name und **absolutem** Server-Pfad (inkl. Ordnerauswahl)
- Register **Uploads**: fortsetzbare Block-Uploads in jeden Serverordner
  (z. B. einen Bibliotheksordner) — fertige Dateien werden automatisch erfasst
- **Scannen** indexiert neue/geänderte Dateien; **Scan stoppen** bricht ab; Fortschritt live
- Das Löschen einer Bibliothek entfernt ihre Videoeinträge — Dateien auf der Platte bleiben
- **Ordner öffnen** öffnet das Mediatheks-Verzeichnis auf dem Server im
  System-Dateimanager, um die tatsächlichen Mediendateien anzusehen oder zu verwalten
- **Bibliothek blockieren** blendet die gesamte Mediathek für alle Benutzer aus
  (Startseite, Serien, Favoriten, Weiterschauen, Wiedergabelisten), ohne etwas
  zu löschen; die Freigabe stellt alles sofort wieder her

### 5.3 Videos

- Videos suchen und Titel/Jahr/Genres/Beschreibung bearbeiten
- Eigenes Poster hochladen
- **Download** öffnet einen Dialog zum Speichern als MKV oder MP4 mit gewählter
  Audiospur und eingebetteten/hochgeladenen Untertiteln (Remux ohne Re-Encoding)
- **Abrufen** holt Metadaten von TMDB (erfordert `TMDB_API_KEY` und Zugriff auf
  api.themoviedb.org)

### 5.4 Benutzer

- Rollen ändern (Benutzer/Admin), Passwörter zurücksetzen, Konten löschen
- Sicherungen: Sie können Ihr eigenes Konto nicht löschen; der letzte Admin kann
  weder gelöscht noch degradiert werden

### 5.5 Blockierte Inhalte

- Medien per Titel blockieren (z. B. Serienname), ohne Dateien oder Einträge zu löschen
- Blockierte Titel sind für alle Benutzer ausgeblendet: Startseite, Serien, Favoriten,
  Weiterschauen und Wiedergabelisten — Freigabe stellt sie sofort wieder her
- Nutzen Sie die Suche, um vor dem Blockieren zu prüfen, welche Medien ein Titel betrifft

### 5.6 Downloads (yt-dlp)

- Beliebige Video-/Playlist-/Kanal-URLs in die Warteschlange; Zielordner,
  yt-dlp-Format und optionales Wiederholungsintervall (Stunden) wählen — der
  Server lädt über yt-dlp herunter
- Live-Fortschritt, Abbrechen und erneuter Versuch fehlgeschlagener Jobs
- Fertige Dateien landen im gewählten Ordner und werden automatisch erfasst,
  wenn er Teil einer Bibliothek ist

## 6. Konfiguration

Alles wird über Umgebungsvariablen konfiguriert (vollständige Tabelle im README). Die wichtigsten:

| Variable | Standard | Zweck |
| --- | --- | --- |
| `PORT` | `8080` | Lauschport |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | Datenbankverbindung |
| `JWT_SECRET` | Dev-Wert | **In Produktion starkes Geheimnis setzen** |
| `TMDB_API_KEY` | leer | Aktiviert Metadaten-Scraping |
| `SCAN_WORKERS` | `4` | Parallele Scan-Worker |
| `WATCH_INTERVAL` | `30` | Sekunden zwischen automatischen Inkrement-Scans; `0` deaktiviert |
| `YTDLP_PATH` | `yt-dlp` im PATH | yt-dlp-Binary für die Download-Warteschlange |
| `HLS_HW_ACCEL` | leer (Software-x264) | HLS-Videoencoder: videotoolbox, nvenc oder qsv; leer = libx264 |

## 7. FAQ

**Warum zeigt meine Mediathek lange „Scan läuft“?**
Der erste Scan eines großen Ordners dauert (ffprobe + Poster je Datei). Der Fortschritt
ist live, und Sie können jederzeit stoppen. Folgescans verarbeiten nur geänderte
Dateien und sind viel schneller. macOS-`._`-Dateien und `.m3u8`-Segmentordner werden automatisch übersprungen.

**Warum kann mein Browser kein MKV/HEVC abspielen?**
Browser dekodieren nur H.264/VP9. VideoCMS transkodiert MKV/HEVC live nach HLS,
oder Sie laden die Datei herunter und spielen sie lokal ab.

**Wie werden Serien erkannt?**
Dateinamen mit `S01E01`, `E01`, `EP1`, `第1集` oder Endzahlen, gruppiert nach gemeinsamem
Präfix. Mindestens 2 Folgen pro Gruppe. Wird Ihr Namensschema nicht erkannt, passen Sie
die Dateinamen an oder bitten Sie um ein neues Muster.

**TMDB-Scraping schlägt fehl.**
Prüfen Sie `TMDB_API_KEY` und den Zugriff auf api.themoviedb.org. In eingeschränkten
Netzen funktioniert nur das Scraping nicht; alles andere bleibt unberührt.

**Wie greife ich vom Handy auf VideoCMS zu?**
Beide Geräte müssen im selben Netz sein. Starten Sie `make serve`, ermitteln Sie die
IP mit `ipconfig getifaddr en0` und öffnen Sie `http://<ip>:8080`. Erlauben Sie beim
ersten Start die macOS-Firewall-Abfrage.

**Ist der öffentliche Zugriff sicher?**
Die Standardinstallation nutzt unverschlüsseltes HTTP mit einem Entwicklungs-JWT.
Für alles außerhalb eines vertrauenswürdigen LAN: HTTPS-Reverse-Proxy und `JWT_SECRET` setzen.

**Wie blende ich Inhalte aus, ohne Dateien zu löschen?**
Admins können einen Titel (Admin → Blockierte Inhalte) oder eine ganze Bibliothek
(Admin → Bibliotheken → Bibliothek blockieren) blockieren; blockierte Inhalte
verschwinden überall für alle Benutzer und kehren nach der Freigabe sofort zurück.
Normale Benutzer können über den Pfadfilter auch beliebige Serverpfade für sich
selbst ausblenden.

## 8. Technik und Lizenz

Go + React + PostgreSQL, mit ffmpeg für die Medienverarbeitung. Die Oberfläche ist
vollständig lokalisiert (en/zh/fr/ja/de). Lizenziert unter **Apache License 2.0**.

Für Interna siehe die [Systemarchitektur](architecture.md).
