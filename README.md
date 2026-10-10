## MediaNinja

### pre-requisites

- ffmpeg
- Go 1.23+

### Build

```bash
go build -o MediaNinja.exe .
```

### Usage

下载单个页面（视频页 / 图片页等）：

```bash
./MediaNinja.exe -u <目标URL> [--proxy http://127.0.0.1:7890] [-c 5] [-o downloads]
```

### rule34video 批量下载

按创作者名或搜索关键词批量下载 rule34video.com 的视频。
优先使用作者页（`/models/<name>/`，支持翻页），作者页不存在时自动回退到搜索页。

```bash
# 每个作者下载前 5 个视频（最高清晰度）
./MediaNinja.exe rule34 -n "juicyneko,mackg4,brutalisk" --limit 5

# 全量下载（不限制数量），作者页抓前 3 页
./MediaNinja.exe rule34 -n "juicyneko,mackg4" --pages 3

# 常用参数
#   -n, --names   逗号分隔的作者名/关键词（必填）
#       --pages   每个名字最多抓取的列表页数（默认 1；仅作者页支持翻页）
#       --limit   每个名字最多下载的视频数（默认 0 = 不限制）
#   -p, --proxy   代理地址（可选）
#   -c, --concurrency  并发下载数（默认 5）
#   -o, --output  输出目录（默认 downloads）
```

输出结构：`downloads/<名字>/videos/<标题> [<视频ID>].mp4`，
每个名字目录下有 `metadata.json` 记录来源与下载链接。
重复执行时已完成的视频会自动跳过（断点续传/秒传检测），可随时调大 `--limit`/`--pages` 续下。
