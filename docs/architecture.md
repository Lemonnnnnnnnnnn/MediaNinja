# 三端复用架构

```mermaid
flowchart LR
  CLI[Cobra CLI] --> APP[Go application]
  WEB[Browser UI] --> HTTP[HTTP / SSE adapter]
  DESKTOP[Desktop shell + shared UI] --> HTTP
  HTTP --> TASKS[Task manager]
  TASKS --> APP
  APP --> CRAWLER[Crawler / site parsers]
  CRAWLER --> DOWNLOAD[HTTP / file / M3U8 / ffmpeg]
```

## 包边界

- `core/application`：单 URL 与 Rule34 作者批量下载用例；校验输入、汇总结果。桌面端无需重写这些规则。
- `core/tasks`：任务 ID、单 worker 队列、状态、取消、结果和进度快照；不依赖 HTTP、Cobra 或前端。
- `core/events`：日志、阶段、字节 / 片段进度和文件成功 / 失败事件。
- `core/crawler`：一次抓取的编排与文件结果；每次执行创建独立 crawler、client 和 parser。
- `core/parsers`：站点解析器；Rule34 列表参数是实例字段。部分解析失败写入 `ParseResult.Failures`。
- `core/request`：传输、下载与合并；任务 client 携带 context 和 observer，子请求复用它们。
- `cmd`：参数解析、终端输出、退出码、服务启动与系统浏览器启动。
- `server`：JSON DTO、会话鉴权、Host / Origin 校验、SSE 和嵌入式静态资源。

## 执行约定

`application.Execute(ctx, request, observer)` 是当前共用入口。调用者负责传入完整默认配置（`config.New()`），或在服务端将 JSON 配置合并到默认值。每个任务独立拥有配置；不修改全局解析器参数。

observer 在一个 crawler 内串行调用，应迅速返回。进度事件按文件组织，普通文件以 bytes、M3U8 以 segments 计量；未知总长度使用 total=0，前端不显示百分比。core 不创建终端进度条，也不向 stdout 打印日志。

请求使用任务 context；重试等待、代理 CONNECT / TLS 和 ffmpeg 也响应取消。取消正在运行的任务先进入 canceling，执行结束后进入 canceled。排队任务取消后不会执行。

任务部分失败时，返回已完成和失败的文件结果，同时返回错误。CLI 返回非零退出码，任务服务记录 failed。错误不再仅通过日志表达。

队列串行执行任务，任务内并发下载，避免同一输出目录被多个 GUI 任务同时写入。独立运行的 CLI 进程之间尚无文件锁；不应同时下载相同内容到同一目录。

## 本地 Web 与桌面适配

共享 React 前端位于 `apps/frontend`，构建产物复制到 `server/ui` 并使用 `go:embed` 打包。默认 loopback 同源提供 UI、API 和 SSE；无需 Node 生产运行时。框架无关的 API 客户端位于 `packages/api-client`，系统能力位于 `packages/platform`。

Tauri 桌面壳启动 `medianinja-core serve --desktop` 子进程，从首行 JSON 读取随机端口服务地址，加载同一 React 页面。外层负责窗口、托盘、文件选择、通知和进程生命周期；页面调用同源会话 API，不需要读取 bearer token。壳持有 stdin 管道，退出或崩溃会触发服务取消任务并关闭。由于会校验 Host，必须使用服务实际打印的地址，不能将 127.0.0.1 随意换成 localhost。

ffmpeg 从 PATH 或可执行文件所在目录查找。分发时 Go exe 与 ffmpeg 仍须按目标 OS / 架构构建或准备。

## 当前范围与下一步

当前任务历史存储在内存中，队列最多 32 个等待任务，最多保留 200 个任务记录（提交新任务时淘汰最早的已结束记录）；服务不提供多用户或公网访问。支持基本取消与已有下载恢复，但没有自动恢复重启前的任务、暂停任务或持久化事件日志。

可以按需求继续增加：任务仓储与 SQLite 持久化、分析预览与下载计划分离、影剧库与外部播放器、远程部署鉴权与存储权限。HTTP 路由与 CLI 继续依赖应用入口，业务规则保持在 Go 中。
