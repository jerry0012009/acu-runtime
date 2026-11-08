# Runtime 0.1.47 / Magpie 0.1.3337

本次配套内核为 v0.1.3337（2026-11-09 确认的上游最新正式版），固定 Windows/macOS amd64、arm64 官方摘要。保留独立安装和手动升级后的内核，不承诺所有未来版本未经测试即可兼容。

发布新增 `scripts/test-kernel-compatibility.py`：使用打包内核和 Runtime，在临时 HOME、Agent 配置与随机回环端口中验证自动更新设置命令、四 Agent 接入与模型写入、共享识图/生图设置、三 MCP HTTP 工具清单、重复配置无重复项、原有 MCP 保留及仅保存共享模型不重写 Agent 文件。使用假 Key，不验证真实推理、登录或外部搜索服务效果。各平台通过情况以对应发行流水线结果为准。

发布平台：Windows 10/11 x64、macOS 12+ Apple Silicon / Intel。macOS 通过对应架构 GitHub runner 原生构建（desktop,production + CGO），不再将服务端交叉编译作为桌面包。

发布流水线对每个平台执行 Go 测试、原生 desktop/production 测试、vet 与前端测试；macOS 另检查应用 Info.plist、ad-hoc 签名、原生桌面进程启动及本地界面。安装包附带固定摘要校验的 Magpie v0.1.3337，路径为 Windows 的应用目录 `kernel/acu-kernel.exe` 或 macOS `.app/Contents/MacOS/kernel/acu-kernel`。

Windows x64 继续支持应用内更新。macOS 通过 DMG 或 ZIP 替换 Applications 内应用；不支持应用内安装。Windows ARM64 尚未发布安装包。当前没有 Windows 代码签名、Apple Developer ID 签名与公证。CI 启动检查不等于真实账号登录、全部 Agent 与多模态业务验收。

下面保留历史验证记录，其范围和版本以各节文字为准。

验证日期：2026-11-06。源码、备用内核与安装包一起升级；不自动采用未经验证的上游 latest。

| 平台 | Runtime 构建 | 官方内核完整下载及 SHA-256 | 运行验证 |
| --- | --- | --- | --- |
| Windows amd64 | 原生桌面成功 | 通过 | 本机覆盖安装、启动、API；隔离备用 CLI 启动与鉴权通过 |
| Windows arm64 | 原生桌面交叉编译成功 | 通过，Go 元数据为 windows/arm64、0.1.3150 | 尚无 ARM64 实机验证 |
| macOS amd64 | 服务端交叉编译成功 | 通过，Go 元数据为 darwin/amd64、0.1.3150 | 尚无 macOS 实机与原生桌面验证 |
| macOS arm64 | 服务端交叉编译成功 | 通过，Go 元数据为 darwin/arm64、0.1.3150 | 尚无 macOS 实机与原生桌面验证 |

校验值固定在 `kernel_install.go`；来自官方 Release 的 SHA256SUMS，并对完整二进制再次计算。不能把交叉编译成功当作其他操作系统上的运行验收。

## 本轮回归

- Go 测试、Windows desktop/production 测试及 vet。
- 78 项前端测试。
- 使用 0.1.3150 原版界面资源完成隔离浏览器回归：模型切换、还原预览、取消零写入、断开确认、首页刷新、托盘三页、展开控件和系统主题。
- Windows 安装器隔离安装成功；安装目录备用内核摘要等于官方 x64 CLI 摘要。
- 本机覆盖安装后 Runtime 报告 0.1.14、ready=true，继续连接独立安装的 Magpie。
- 备用 CLI 在隔离用户目录中启动，管理接口鉴权及 `/api/state` 读取成功；未发送收费推理请求。
- 回归测试覆盖旧备用内核替换、用户文件保留，以及校验失败时原安装不变。
- 应用内更新只下载仓库的正式版本，核验 GitHub Release SHA-256，安装器核验随包内核版本及摘要，等待 Runtime 退出后更新并重启；覆盖回滚、版本选择、平台资产与前端控制测试。

## 更新边界

安装包每次都更新自己的备用内核，不再因检测到外部 Magpie 而跳过。现有外部 Magpie、Key 与 Agent 配置不由安装器改写。Runtime 的外部内核选择仍遵循原有发现规则；显式 ACU_KERNEL_PATH 也保留。

当前应用内安装包只支持 Windows x64。macOS 使用 0.1.32 新增的独立 DMG / ZIP 安装包，应用内不会执行下载安装；Windows ARM64 尚无对应安装包。独立 Magpie 不由 Runtime 安装器覆盖或降级。0.1.39 起连接前使用内核 update auto off notify 和 settings set autoRestart off 关闭自动更新；手动升级保留。

更新网络：优先使用显式 HTTP(S)_PROXY/NO_PROXY 环境设置，否则读取 Windows 手动系统代理。PAC 自动代理与国内镜像尚未接入。无法访问 GitHub 时保留现有安装，可稍后重试。

## 0.1.16 在线更新验收

- 本机真实 0.1.15 安装经正式 GitHub Release 检查、下载、校验后更新到 0.1.16；旧进程退出，新进程自动启动并返回 ready=true。
- 更新后的 Runtime 可执行文件与发布构建摘要一致，备用内核摘要对应 Magpie 0.1.3150。
- 更新前后 Codex、Claude、Magpie providers/settings 共 4 个关键配置文件 SHA-256 不变。
- 旧版测试进程临时设置 HTTPS_PROXY 以使用本机已有代理；0.1.16 随后在无此临时环境变量的新进程中，通过 Windows 手动系统代理完成检查并报告 current。
- Windows ARM64 桌面和 macOS amd64/arm64 服务端交叉编译通过；不代表其他平台的实机运行验收。
- 0.1.15 已发布但不读取 Windows 手动系统代理，建议新用户安装 0.1.16。国内无 GitHub 网络的用户仍需后续官方镜像；当前未接入公共代理或镜像。


## 0.1.17 公开代理自动备用

检查版本依次尝试 GitHub、gh-proxy.com，每条线路最多 12 秒。下载安装包依次尝试 GitHub（最多 45 秒）、gh-proxy.com、ghfast.top（代理每条最多 5 分钟）。失败或摘要不符自动换线；所有线路均验证同一 SHA-256，安装器继续验证内核摘要。仅访问公开发布资源，不传递用户 Key 或 GitHub 凭据。未引入签名清单或自建镜像。已有系统代理支持保留。

测试覆盖主线路失败、版本查询备用、下载备用、错误内容被拒绝以及取消后不再换线。


### 0.1.34 图片与搜索

- 通过固定内核 v0.1.3150 的共享设置和 Library API 配置；未修改内核或全局 MCPHub。
- Windows 隔离配置目录中实测四个 Agent 写入（无搜索后端时仅添加免费搜索 MCP，不添加无后端的 ACU 搜索 MCP）：Claude `.claude.json`、Codex `config.toml`、Hermes `config.yaml`、WorkBuddy `mcp.json`。重复配置无重复条目，原有 MCP、共享显式模型、禁用工具与 hub 选项保留；共享模型切换成功。
- DuckDuckGo MCP 0.7.0 `[browser]` 通过 stdio 初始化、tools/list 和真实搜索，返回 Python 官网文档结果。GitHub 项目 2026-11-08 查询约 1,572 stars，MIT 许可。安装包附带经过 SHA-256 校验的官方固定 uv 0.13.7，缺少环境时自动准备，无需用户手动安装；需要网络，尚未完成每个平台每个 Agent 的交互调用验收。
- 新增官方运行环境压缩包 SHA-256 校验、ZIP / tar.gz 限定可执行文件解压、缺包拒绝测试；前端新增默认启用 / 取消、只提交显式选模、失败重试测试。Go 覆盖模型、域名和所有权校验、部分写入失败、Key 隔离与重复配置。浏览器检查深色和 390px 浅色布局、保存及模型选择。
- ACU SearXNG 服务无已上线地址，不能标为已可用。服务须支持 SearXNG JSON 和 Bearer ACU Key。无付费推理自动测试；工具注册不代表所有模型或完整 PDF 能力验收通过。

- 无 uvx 的 Windows 新配置目录验收：包内校验解压私有运行环境、自动下载 MCP 依赖、四 Agent 分配、重复执行和模型切换全链路通过。Python 运行版本固定为 3.12，由 uv 自动准备，避免系统 Python 版本差异。


### 0.1.36 配置交互

- 模型单独保存模式不读取 Agent 状态、不依赖网关、不下载搜索环境、不重写 MCP；只写选中的字段。禁止混合工具配置请求。
- 重复一键配置的 preserve 模式保留已接入应用主模型和可见性；首次接入仍按推荐配置，用户可显式选择替换。
- UI 覆盖已配置状态无开关、模型 / 工具操作隔离、失败刷新和定向重试、高级搜索跳转内核与自定义标记。
- 浏览器验证 1280px 深色、390px 浅色、小高度窗口、模型保存和一键配置选择，底部按钮可访问。


### 0.1.37 独立管理页

- 图片与搜索改为独立页；模型局部表单与高级入口分离，合并同页内核设置入口。
- 前端测试覆盖离开时保存 / 放弃 / 继续编辑、恢复原值消除修改、保存失败保留草稿与跳转隔离。
- 浏览器验证真实点击保存后进入内核、返回重新读取、侧栏导航放弃修改、390px 浅色布局与独立模型表单结构。


### 0.1.38 统一工具检查

- 通用工具统一检查，取消逐应用常驻配置按钮；单独重试仅限本次检查失败应用。
- 前端覆盖逐应用请求、失败原因展示、成功保留、定向重试、重试成功移除按钮与共享模型草稿保留。

## 0.1.39 内核复用与更新

优先读取 Magpie web-page.json 的已启用管理服务，在回环地址鉴权并验证 /api/state 后复用。未开启或连接失败时才启动原生 web 控制进程；原生内核复用已运行的推理网关。支持 web 命令返回已有服务链接后正常退出的场景，不记录管理密钥。退出只终止自己启动的控制进程。

自动更新开关通过 Magpie 原生命令修改，保留其余设置。独立安装保持原路径和版本，不自动降级。该策略不保证用户手动升级后的新版本兼容。已在更新前下载暂存的升级不由 Runtime 清除。

0.1.39 本机隔离实测：已有管理服务时 ready=true、Runtime 常驻 Magpie 子进程为 0；无管理服务时 ready=true、控制子进程为 1。两条路径均确认 noAutoUpdate=true、noAutoDownload=true、autoRestart=false。测试没有发送收费推理请求。

## 0.1.41 后台可见性与路由说明

Windows 后台进程设置 HideWindow 与 CREATE_NO_WINDOW，保留其余进程属性。随包内核官方 CLI 的 Go 构建标签为 nogui；不启动 gui/tray、不创建 Magpie 快捷方式或桌面自启。隔离配置使用随包 CLI 实测：管理服务复用时无常驻子进程，备用控制路径一个子进程且 MainWindowHandle=0、ready=true。本机已有独立桌面 Magpie 保留。

路由跟踪确认原生 OpenAI 与 ACU 请求属于不同 session；一键配置不重写已存在的聊天选模。推荐预览与路由页说明保留聊天选模及按模型来源路由的行为。
