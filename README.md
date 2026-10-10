## MediaNinja

### pre-requisites

- ffmpeg
- Go 1.23+（构建）
- Node.js 22.12+、pnpm 10.18.0（构建前端；运行成品不需要）

### Build

```bash
corepack enable
pnpm install
pnpm build:web
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


### 本地 Web 控制台

构建后双击 `MediaNinja.exe`（无参数启动）会开启本机 HTTP 服务，并打开默认浏览器。前端资源已经嵌入程序，不需要安装 Node.js。

也可以显式启动：

```powershell
.\MediaNinja.exe serve --open
.\MediaNinja.exe serve --listen 127.0.0.1:8080 -o downloads
```

默认监听 `127.0.0.1:0`，由系统分配空闲端口。命令会打印访问地址与本次运行的 API token。

- 页面支持 URL 下载和 Rule34 作者 / 关键词批量下载，以及进度查看、任务取消。
- 一个任务执行队列，任务内按 `--concurrency` 并发下载；配置中心可以修改目录、代理、并发和重试设置，只影响新任务。
- 关闭浏览器不会停止任务；在终端按 Ctrl+C 停止服务会取消任务。
- 任务历史保存在内存中，重启后清空。普通文件和 M3U8 的未完成数据保留在磁盘，可以重新提交任务续传。
- 当前只允许绑定 loopback IP，适合本地使用和桌面壳接入。远程部署的鉴权与多用户隔离尚未实现。
- M3U8 合并仍需 ffmpeg：可以安装到 PATH，或将 `ffmpeg.exe` 放在 `MediaNinja.exe` 旁边。

CLI 命令仍可直接调用 core，不需要先启动服务。CLI 执行时按 Ctrl+C 可以取消请求、重试等待与 ffmpeg 合并；部分文件失败会返回非零退出码。

### HTTP API

浏览器访问页面后使用 HttpOnly 会话 Cookie；其他客户端使用服务启动时打印的 token：

```powershell
$headers = @{ Authorization = 'Bearer <启动时打印的 token>' }
$body = @{ url = 'https://telegra.ph/<page>' } | ConvertTo-Json
Invoke-RestMethod -Uri http://127.0.0.1:8080/api/tasks -Method Post -Headers $headers -ContentType application/json -Body $body
```

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/session` | 初始化同源浏览器会话 |
| GET | `/api/config` | 当前服务配置 |
| PUT | `/api/config` | 校验并更新会话配置 |
| POST | `/api/tasks` | 创建任务，返回 202 和任务快照 |
| GET | `/api/tasks` | 查询任务列表 |
| GET | `/api/tasks/{id}` | 查询任务详情 |
| POST | `/api/tasks/{id}/cancel` | 取消任务 |
| GET | `/api/events` | SSE：`tasks` 事件携带完整列表快照 |

提交请求使用 `url` 或 `names` 二选一，`options` 可省略；省略的设置继承服务默认值。目前配置字段使用 Go 字段名，如 `Concurrency`、`ProxyURL`、`MaxPages`、`MaxVideos`。`OutputDir` 通过配置 API 修改；提交任务不能覆盖输出目录。每个任务保留提交时的配置快照。

SSE 建立连接时立即返回快照，随后每秒更新；重连和页面刷新会重新读取当前状态，不依赖重放历史事件。任务状态为 `queued`、`running`、`canceling`、`completed`、`failed`、`canceled`。

### 开发与测试

```powershell
go test ./...
go vet ./...
corepack enable
pnpm install
pnpm build:web
```

默认测试使用本地 HTTP 测试服务，不访问真实媒体网站。原有依赖外部站点及本机 `7890` 代理的测试改为显式运行：

```powershell
go test -tags integration ./core/request/client
```

架构与后续适配边界见 [docs/architecture.md](docs/architecture.md)。Web 与 Windows 桌面端共用 Vite + React + TypeScript + Tailwind CSS 前端，桌面壳使用 Tauri 2 与 Go sidecar。开发、构建和分发见 [docs/desktop.md](docs/desktop.md)。
