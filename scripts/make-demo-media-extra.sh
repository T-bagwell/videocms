#!/usr/bin/env bash
#
# Generate demo audio, photo and book material for screenshots and manuals.
# Requires ffmpeg; produces files under demo-media/music, demo-media/photos
# and demo-media/books alongside the video demo media.
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)/demo-media"
FFMPEG="${FFMPEG:-/usr/local/opt/ffmpeg/bin/ffmpeg}"

# --- music ---------------------------------------------------------------
gen_audio() {
  local out="$1" artist="$2" album="$3" title="$4" freq="$5"
  if [[ -f "$out" ]]; then return; fi
  mkdir -p "$(dirname "$out")"
  "$FFMPEG" -y -loglevel error \
    -f lavfi -i "sine=frequency=$freq:duration=8" \
    -c:a libmp3lame -b:a 96k \
    -metadata "artist=$artist" -metadata "album=$album" -metadata "title=$title" \
    "$out"
}

gen_audio "$ROOT/music/Retro Nights/Night City.mp3" "Synthwave" "Retro Nights" "Night City" 440
gen_audio "$ROOT/music/Retro Nights/Neon Drive.mp3" "Synthwave" "Retro Nights" "Neon Drive" 523
gen_audio "$ROOT/music/Retro Nights/Midnight Run.mp3" "Synthwave" "Retro Nights" "Midnight Run" 587
gen_audio "$ROOT/music/Morning Beats/Sunrise.mp3" "Lofi" "Morning Beats" "Sunrise" 330
gen_audio "$ROOT/music/Morning Beats/Paper Planes.mp3" "Lofi" "Morning Beats" "Paper Planes" 392

# Album cover for Retro Nights (embedded into one track so the album shows art).
COVER="$ROOT/music/Retro Nights/cover.jpg"
mkdir -p "$(dirname "$COVER")"
if [[ ! -f "$COVER" ]]; then
  "$FFMPEG" -y -loglevel error -f lavfi -i "gradients=size=600x600:speed=0.01:c0=0x1a1a2e:c1=0xe50914" \
    -frames:v 1 "$COVER"
fi
TAGGED="$ROOT/music/Retro Nights/Night City (cover).mp3"
if [[ ! -f "$TAGGED" ]]; then
  "$FFMPEG" -y -loglevel error -i "$ROOT/music/Retro Nights/Night City.mp3" -i "$COVER" \
    -map 0:a -map 1:v -c:a copy -c:v mjpeg -id3v2_version 3 \
    -metadata "artist=Synthwave" -metadata "album=Retro Nights" -metadata "title=Night City" \
    "$TAGGED"
fi

# --- photos --------------------------------------------------------------
gen_photo() {
  local out="$1" color="$2" label="$3"
  if [[ -f "$out" ]]; then return; fi
  mkdir -p "$(dirname "$out")"
  "$FFMPEG" -y -loglevel error \
    -f lavfi -i "color=c=$color:size=800x600" -frames:v 1 "$out"
}

gen_photo "$ROOT/photos/Vacation/beach.jpg" 0x2f6ed8 "beach"
gen_photo "$ROOT/photos/Vacation/sunset.jpg" 0xf39c12 "sunset"
gen_photo "$ROOT/photos/Vacation/palms.jpg" 0x27ae60 "palms"
gen_photo "$ROOT/photos/City Night/skyline.jpg" 0x2c3e50 "skyline"
gen_photo "$ROOT/photos/City Night/neon.jpg" 0xe50914 "neon"
gen_photo "$ROOT/photos/City Night/rain.jpg" 0x34495e "rain"

# --- books ---------------------------------------------------------------
mkdir -p "$ROOT/books"
if [[ ! -f "$ROOT/books/Novel.epub" ]]; then
  python3 - "$ROOT/books/Novel.epub" <<'PY'
import sys, zipfile
path = sys.argv[1]
with zipfile.ZipFile(path, "w") as z:
    z.writestr("mimetype", "application/epub+zip")
    z.writestr("META-INF/container.xml", """<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>""")
    z.writestr("OEBPS/content.opf", """<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>The Long Night</dc:title><dc:creator>Demo Author</dc:creator>
  </metadata>
  <manifest>
    <item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="c1"/></spine>
</package>""")
    z.writestr("OEBPS/ch1.xhtml", """<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter 1</title></head>
<body><h1>Chapter 1</h1><p>It was a long night in the city of lights.</p></body></html>""")
PY
fi

if [[ ! -f "$ROOT/books/Comic.cbz" ]]; then
  python3 - "$ROOT/books/Comic.cbz" "$ROOT/photos/City Night" <<'PY'
import sys, zipfile, os
path, src = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(path, "w") as z:
    for n in sorted(os.listdir(src)):
        z.write(os.path.join(src, n), f"page {n}")
PY
fi

if [[ ! -f "$ROOT/books/Manual.pdf" ]]; then
  python3 - "$ROOT/books/Manual.pdf" <<'PY'
import sys
path = sys.argv[1]
pdf = b"""%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 400 300]/Contents 4 0 R>>endobj
4 0 obj<</Length 60>>stream
BT /F1 18 Tf 30 160 Td (VideoCMS Manual) Tj ET
endstream
endobj
5 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000210 00000 n 
0000000333 00000 n 
trailer<</Root 1 0 R/Size 6>>
%%EOF
"""
open(path, "wb").write(pdf)
PY
fi

echo "Extra demo media ready:"
du -sh "$ROOT"/music "$ROOT"/photos "$ROOT"/books 2>/dev/null
