# Remux fixture

`remux.ts` is a generated one-second, 16×16 black H.264 video in MPEG-TS,
with encoder SEI removed. It contains no third-party media.

Regenerate using a full development FFmpeg with libx264 (the shipped remux
build intentionally does not include this encoder):

```text
ffmpeg -f lavfi -i color=c=black:s=16x16:r=1 -t 1 -c:v libx264 -preset ultrafast -tune zerolatency -bsf:v filter_units=remove_types=6 -f mpegts -y remux.ts
```
