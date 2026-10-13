# Windows 版本与分发

产品版本以根目录 `package.json` 为准，桌面配置、Cargo.toml 与 Cargo.lock 的 `medianinja-desktop` 版本必须一致。workspace 内部包不独立发布，无需跟随产品版本变化。

## 本地准备

需要 Windows x64、Go、Node.js 22.12+、pnpm 10.18.0、Rust MSVC、Visual Studio C++ Build Tools、Windows SDK 和 Git for Windows（包含 Git Bash）。构建时需要网络，使用时无需下载 ffmpeg。

```powershell
pnpm install --frozen-lockfile
pnpm version:set 0.2.0
pnpm version:check
pnpm release:build
```

版本脚本支持标准三段版本及 prerelease，例如 `0.2.0-beta.1`，只同步版本文件，不执行 commit、tag 或 push。Go 版本、提交哈希和构建时间由构建脚本通过 `-ldflags` 注入。普通 `go build` 显示 `dev`。需要固定构建时间时可设置 Unix 秒数形式的 `SOURCE_DATE_EPOCH`。

## 发布流程

完成代码与版本变更后提交到 master，确认所有需要发布的代码已提交，再创建版本 tag：

```powershell
git tag -a v0.2.0 -m "MediaNinja v0.2.0"
git push origin master
git push origin v0.2.0
```

`release.yml` 在 Windows runner 中执行与本地相同的 `pnpm release:build`，校验版本与 tag 一致，随后创建 **Release 草稿**。检查附件和说明后手动发布。包含 prerelease 的 tag 会标记为预发行版。手动运行 workflow 的分支构建只上传 Actions artifacts，不创建 Release。

本地与 CI 都执行前端类型检查、发布脚本测试、Go 测试与 vet、ffmpeg 媒体合并验证、桌面构建，以及 Go sidecar 的就绪协议、页面资源、API 和退出验证。当前 Windows 安装包未签名，没有自动更新。

## 分发文件

| 文件 | 用途 |
| --- | --- |
| `MediaNinja_<version>_x64-setup.exe` | 桌面安装包，包含 Tauri、Go sidecar 和 ffmpeg |
| `MediaNinja_<version>_windows_x64.zip` | 解压即用的本地 Web 与 CLI，包含 ffmpeg |
| `SHA256SUMS.txt` | 安装包、便携包、源码和构建配方附件的 SHA256 |
| `ffmpeg-<version>.tar.xz` | 与二进制对应的未修改 FFmpeg 官方源码 |
| `ffmpeg-build.json`、`build-ffmpeg.sh` | 固定源码与工具链信息、编译参数及构建脚本 |

用户优先下载安装包或便携版。GitHub 自动生成的 Source code 是项目源码，不是可运行的软件。安装程序会处理 WebView2 缺失的情况；下载服务使用本机随机端口。设置和历史持久化到每用户配置目录（settings.json 与 tasks.json）。

## ffmpeg 构建与许可证

`scripts/ffmpeg.lock.json` 锁定官方源码和 w64devkit 编译工具链的 URL、版本和 SHA256。`pnpm prepare:ffmpeg` 先校验下载文件，再解压和构建。缓存会校验构建配方和产物哈希；二进制、缓存、工具链和发行包不提交到 Git。

这是面向当前 MPEG-TS → MP4 合并需求的精简 ffmpeg：启用需要的本地输入、容器、解析器和 AAC 转换，禁用外部依赖自动检测、网络、GPL 与 nonfree 组件。它不是完整的通用 ffmpeg 工具；后续增加转码或其他容器能力时，需要更新配方和对应媒体测试。工具链只用于构建，不随应用分发。

安装包与便携版都在 `licenses/ffmpeg` 中包含 LGPL 文本、对应未修改源码、构建参数、脚本和来源说明；Release 还单独提供源码与配方附件。参见 [FFmpeg 官方许可证说明](https://ffmpeg.org/legal.html)。

Go 查找顺序为：`MEDIANINJA_FFMPEG` 明确指定的文件 → Go 程序旁的 ffmpeg → 系统 PATH。显式路径无效会报错，不会悄悄回退；桌面默认使用随包提供的版本。桌面开发的 Go sidecar 目录也会放置 ffmpeg。

```powershell
pnpm prepare:ffmpeg
pnpm test:ffmpeg
```

首次构建耗时较长；后续复用 `.cache/ffmpeg`。升级 ffmpeg 时更新源码版本与可信 SHA256，重新构建并验证，再发布产品新版本。
