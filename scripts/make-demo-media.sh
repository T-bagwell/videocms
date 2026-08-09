#!/usr/bin/env bash
#
# 生成演示视频素材（无需真实片源即可体验系统）：
# 用 ffmpeg 的合成画面 + 正弦音频生成几个短视频。
#
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)/demo-media"
mkdir -p "$DIR"

gen() {
  local name="$1" src="$2"
  local out="$DIR/$name.mp4"
  if [[ -f "$out" ]]; then
    echo "skip (exists): $out"
    return
  fi
  ffmpeg -y -loglevel error \
    -f lavfi -i "$src" \
    -f lavfi -i "sine=frequency=440:duration=15" \
    -t 15 -c:v libx264 -preset veryfast -crf 28 -pix_fmt yuv420p \
    -c:a aac -shortest "$out"
  echo "generated: $out"
}

gen "星际穿越 Interstellar (2014)" "testsrc2=size=1280x720:rate=24"
gen "盗梦空间 Inception (2010)" "smptebars=size=1280x720:rate=24"
gen "千与千寻 Spirited Away (2001)" "mandelbrot=size=1280x720:rate=24"

echo
echo "Demo media ready at: $DIR"
ls -lh "$DIR"
