# MediaNinja

MediaNinja 是一个媒体下载工具，提供 CLI、本地 Web 工作台和 Windows 桌面端。三端共用 Go 核心；Web 与桌面共用 React 前端，桌面通过 Tauri 2 管理窗口、托盘和本地 Go 服务。

工作台支持创建下载任务、查看进度、取消任务、会话历史，以及输出目录、代理、下载引擎和解析器配置。界面使用英语，默认暗色主题。

## 开发环境

以下命令在仓库根目录执行，以 Windows PowerShell 为例。

| 工具 | 要求 | 用途 |
| --- | --- | --- |
| Go | 1.23+ | CLI、HTTP 服务和下载核心 |
| Node.js | 22.12+ | 前端开发与构建 |
| pnpm | 10.18.0 | 管理前端和桌面 workspace |
| ffmpeg | 在 PATH 中，或放在 Go 可执行文件旁 | M3U8 合并；不随安装包分发 |

桌面开发和构建还需要 Rust MSVC 工具链、Visual Studio C++ Build Tools、Windows SDK 和 WebView2 运行时。当前桌面版本以 Windows x64 为目标。

安装前端与桌面工具依赖：

```powershell
corepack enable
pnpm install
```

仅开发 CLI 时，不需要 Node.js、pnpm 或 Rust。

## Dev

### 本地 Web

```powershell
pnpm dev
```

该命令编译并启动 Go 服务，再启动 Vite。打开 <http://127.0.0.1:5173>，前端通过 Vite 代理访问 Go API。

React 页面修改支持热更新；修改 Go 代码后需要停止并重新运行 `pnpm dev`。按 Ctrl+C 会停止前端和 Go 服务，并取消运行中的下载任务。

也可以分别启动后端和前端，在两个终端中执行：

```powershell
# 终端 1：Go API
go run . serve --listen 127.0.0.1:43189
```

```powershell
# 终端 2：Vite，默认代理到上述端口
pnpm dev:web
```

如果后端使用其他地址，在启动 Vite 前设置 `$env:MEDIANINJA_BACKEND_URL = 'http://127.0.0.1:你的端口'`。

### Windows 桌面端

```powershell
pnpm desktop:dev
```

命令会构建前端与 Go sidecar，再启动 Tauri。桌面壳加载同一套 React 页面，并负责启动、停止本地 Go 子进程。

当前桌面开发加载前端构建目录：

- 修改前端后，执行 `pnpm build:frontend`，在桌面窗口按 Ctrl+R 刷新。
- 修改 Go 后，执行 `pnpm build:sidecar`，再重启桌面开发进程。
- 需要频繁调整页面时，使用 `pnpm dev` 获得 Vite 热更新，随后在桌面窗口验证原生功能。

### CLI

```powershell
go run . --help
go run . -u '<目标页面 URL>' -o downloads
```

CLI 直接调用 Go 核心，无需先启动 HTTP 服务。

## Build 与分发

### 本地 Web 可执行文件

```powershell
pnpm build:web
```

该命令构建 React，将资源复制到 `server/ui`，再通过 `go:embed` 编入根目录的 `MediaNinja.exe`。分发这个文件即可，用户无需安装 Go 或 Node.js。

双击 `MediaNinja.exe` 会启动本地服务并打开默认浏览器，也可以显式启动或指定端口：

```powershell
.\MediaNinja.exe serve --open
.\MediaNinja.exe serve --listen 127.0.0.1:8080 -o downloads
```

默认使用系统分配的空闲端口，只监听本机 loopback 地址。关闭浏览器后任务仍会继续；在服务终端按 Ctrl+C 退出。

### Windows 桌面安装包

```powershell
pnpm desktop:build
```

该命令自动构建前端、Go sidecar、Tauri 程序和 NSIS 安装包。分发以下目录中的 `*-setup.exe`：

```text
apps/desktop/src-tauri/target/release/bundle/nsis/
```

安装包包含桌面程序与 Go sidecar；用户无需安装 Go、Node.js 或 Rust。WebView2 缺失时由安装器引导安装。当前安装包未签名，ffmpeg 需要另行提供。

桌面窗口关闭时隐藏到托盘；使用托盘菜单的 Quit 退出程序并停止下载服务。

### 仅构建 CLI

```powershell
go build -o MediaNinja.exe .
.\MediaNinja.exe -u '<目标页面 URL>' -o downloads
```

直接 `go build` 不会构建或更新前端资源。需要可分发的本地 Web 程序时，使用 `pnpm build:web`。

### 辅助构建命令

| 命令 | 产物 |
| --- | --- |
| `pnpm build:frontend` | `apps/frontend/dist`，仅构建前端 |
| `pnpm build:sidecar` | 嵌入前端的 Windows Go sidecar，输出到 `apps/desktop/src-tauri/binaries`；需要 Rust MSVC 工具链 |

## 检查

```powershell
pnpm typecheck
go test ./...
go vet ./...
```

默认 Go 测试使用本地测试服务，不访问真实媒体网站。桌面环境还可以验证 sidecar 的启动协议、API、嵌入资源和退出行为：

```powershell
pnpm build:sidecar
pnpm test:sidecar
```

## 项目结构

| 目录 | 职责 |
| --- | --- |
| `core` | 解析、请求、下载等 Go 核心能力 |
| `cmd` | CLI 入口与命令适配 |
| `server` | 本地 HTTP API、任务队列和前端资源嵌入 |
| `apps/frontend` | Web 与桌面共用的 Vite、React、TypeScript、Tailwind CSS 前端 |
| `apps/desktop` | Tauri 2 桌面壳与原生能力 |
| `packages/api-client` | HTTP 客户端、类型和 SSE 订阅 |
| `packages/platform` | 浏览器与桌面的平台能力适配 |
| `scripts` | 开发启动、构建和 sidecar 验证脚本 |

当前配置与任务历史保存在会话内存中，重启后恢复默认配置并清空历史。配置修改只影响新任务。服务面向本机使用，暂不支持远程多用户部署。

架构边界见 [架构说明](docs/architecture.md)，桌面实现与开发细节见 [Web 与 Windows 桌面开发](docs/desktop.md)。
