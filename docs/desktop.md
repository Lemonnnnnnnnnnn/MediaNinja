# Web 与 Windows 桌面开发

项目使用 pnpm workspace 管理前端与桌面壳，Go module 管理业务代码。

| 目录 | 职责 |
| --- | --- |
| `apps/frontend` | Vite、React、TypeScript、Tailwind CSS 4、shadcn/ui 风格的源码组件；Web 和桌面共用 |
| `apps/desktop` | Tauri 2 Windows 壳：窗口、托盘、目录选择、通知、Go 子进程生命周期 |
| `packages/api-client` | HTTP DTO、API 调用与 SSE 订阅；不依赖 UI 框架 |
| `packages/platform` | HostCapabilities；浏览器实现与 Tauri 原生桥接 |
| `core` / `server` | Go 用例、下载任务与本地 HTTP 服务 |

开发需要 Go 1.23+、Node.js 22.12+（建议 22 LTS）以及 pnpm 10.18.0。桌面开发还需要 Rust MSVC 工具链、Visual Studio C++ Build Tools、Windows SDK 和 WebView2。

```powershell
corepack enable
pnpm install
pnpm dev
```

`pnpm dev` 自动编译并启动 Go，Vite 使用随机分配的 Go 端口代理 API，访问 http://127.0.0.1:5173。结束开发进程会关闭服务。仅开发前端时可运行 `pnpm dev:web`，通过 `MEDIANINJA_BACKEND_URL` 指定已经启动的 Go 服务。

```powershell
pnpm desktop:dev
pnpm build:web
pnpm desktop:build
```

桌面开发会先构建 React 和 Go sidecar，再启动 Tauri。当前桌面开发加载 React 的构建目录；修改页面后执行 `pnpm build:frontend`，在桌面窗口按 Ctrl+R 刷新。浏览器开发使用 Vite 热更新。

`build:web` 将 React 构建结果复制到 `server/ui`，通过 go:embed 编入根目录的 `MediaNinja.exe`。终端用户无需 Node.js。双击 exe 打开本地 Web，也可继续使用原有 CLI 命令。直接 `go build` 不会生成前端资源，适合 CLI 开发；没有生成资源时 Web 页面提示先构建前端。

`desktop:build` 自动生成 Go sidecar 和 Windows NSIS 安装包，位于 `apps/desktop/src-tauri/target/release/bundle/nsis/`。安装包包含桌面 exe 与 Go sidecar。未安装 WebView2 的机器会由安装器引导获取运行时。当前安装包未签名。M3U8 合并还需要 ffmpeg，可放到 PATH 或 Go sidecar 旁边；未打包 ffmpeg。

桌面关闭按钮隐藏到托盘；托盘菜单可以重新打开窗口，或退出并停止任务。退出时关闭 Go 标准输入，Go 收到 EOF 后取消任务并关闭 HTTP 服务；异常结束桌面进程也会断开标准输入。服务仅监听 loopback 随机端口，页面使用 HttpOnly 会话 Cookie。原生命令同时受 Tauri capability 和 Rust 的精确 origin 校验约束。

当前支持下载、进度与取消、会话历史、会话配置（目录、代理、并发、重试和 Rule34 参数）。配置保存只影响新任务，重启恢复启动默认值；历史重启清空。SQLite 持久化、影剧库、mpv/VLC 启动与播放控制还未实现。后续把持久化和播放器管理加到 Go 用例与 HTTP API，页面继续共用；只有必须使用系统能力的功能才扩展 HostCapabilities。

```powershell
pnpm typecheck
go test ./...
go vet ./...
pnpm build:sidecar
pnpm test:sidecar
```

sidecar 测试检查就绪协议、会话、API、嵌入的 React 资源以及父进程关闭后的退出行为，不访问真实媒体网站。

界面使用英语和默认暗色的 neobrutalism 功能工作台：新建下载、目录与引擎参数、任务队列共同占据首屏，不使用宣传主视觉。队列支持 Active、This session、Failed 筛选。配置与会话历史通过浮动面板打开，支持 Escape 关闭、键盘焦点约束和窄屏排列。配置中心按存储、下载引擎、解析器独立分组；Rule34 提供列表页数和视频上限，其他解析器仍使用内置默认值。

Paper Mono 变量字体从 https://paper.design/mono/download 获取，保存在 `apps/frontend/public/fonts`，随 Web 和桌面成品一起离线分发。SIL OFL 1.1 许可证一并保留；界面运行时无需外部字体 CDN。
