# ACU Runtime 0.1.38

Wails 原生应用 + 独立内核服务，不需要浏览器。关闭窗口驻留托盘，再次启动唤醒同一实例。

## 安装与使用

从 [GitHub Releases](https://github.com/jerry0012009/acu-runtime/releases/latest) 下载对应平台安装包。完整安装说明见 [RELEASE-NOTES.md](RELEASE-NOTES.md)。

| 平台 | 安装包 | 安装 |
| --- | --- | --- |
| Windows 10/11 x64 | `ACU-Runtime-0.1.38-windows-amd64-Setup.exe` | 双击安装到当前用户目录，开始菜单打开；无需管理员权限 |
| macOS 12+ Apple Silicon | `ACU-Runtime-0.1.38-macos-arm64.dmg` | 将 ACU Runtime 拖入 Applications |
| macOS 12+ Intel | `ACU-Runtime-0.1.38-macos-amd64.dmg` | 将 ACU Runtime 拖入 Applications |

Windows 依赖 Microsoft Edge WebView2 Runtime。macOS 同时提供 `.app.zip`。应用尚无 Windows 代码签名及 Apple Developer ID 公证；首次打开的系统提示及处理方式见安装说明。macOS 使用 ad-hoc 签名。Windows ARM64 暂未提供安装包。

安装包自带 Magpie `v0.1.3150` 备用内核。Runtime 优先连接本机已有 Magpie；安装或更新 ACU Runtime 时都会刷新安装目录内的备用内核，既有 Agent 配置保留。找不到独立 Magpie 时使用此备用内核；单独运行未附带内核的应用时，首次启动会从官方 Release 下载并校验 SHA-256。内核包支持 Windows / macOS 的 amd64、arm64；macOS 原生桌面在对应架构 runner 构建并进行启动检查；真实账号与 Agent 业务验证需在对应设备完成。

老用户可以在“设置 → 应用更新”一键检查并更新。Runtime 从 ACU Runtime 正式版读取发布信息；下载与安装包的 SHA-256 相符才继续。确认后安装器等待当前进程退出，一起更新 Runtime 与它管理的备用内核，再自动重启。失败时保留或恢复旧程序和内核；Key、Agent 与本地配置保留。独立安装的 Magpie 不会被安装器覆盖或降级；连接前调用 Magpie 原生命令关闭自动检查、自动下载和自动重启更新，用户仍可在 Magpie 中手动升级。每个 Runtime 发行版固定经过检查的备用内核版本，详细验证范围见 [COMPATIBILITY.md](COMPATIBILITY.md)。

打开后自动连接，无需找路径、选择管理端口或确认启动。优先通过 Magpie 的本机浏览器访问记录连接已运行的管理服务；服务未开启或不可用时，才启动必要的 web 控制进程。Magpie 原生逻辑复用已有推理网关，控制进程不等于第二套推理网关；没有网关运行时才接管。失败才显示原因及“重试”。从托盘退出只停止本次启动的管理进程，不停止复用的外部服务或原有 Magpie。

## 已接入

- 托盘快捷面板：单击右下角 ACU 图标弹出原生无边框窗口，复用内核的 Agent / 用量 / 路由三页；点外部收起，右键菜单打开主窗口或退出。面板内“打开主窗口”进入对应详情，窗口操作不会发送给原有内核 GUI。
- 工作台：新用户同时看到三步引导和 Agent 工作台；已有连接直接显示完成状态。登录、Key 选择和连接管理在同一页，账号与手动 Key 管理按需展开。
- 主界面：导航只保留工作台、用量、请求记录、设置。Agent 状态按原生配置、ACU 路由、应用内选模和重启要求表达；单个 Agent 接入先确认再写入。
- 单 Agent 接入：卡片直接打开该应用的推荐配置，兼容模型中优先选 acu-auto；查看默认路由、原有与 ACU 模型、能力影响后确认。只写入所选 Agent，不重复选择应用，也不展示辅助模型同步开关。WorkBuddy 不设默认模型，保留任务内选择。顶部入口命名为“推荐配置”，按推荐值完成首次接入；需要细调时再进入“高级配置”。
- 接入开关：卡片上方的大开关管理整个 Agent 的本地接入，不代表所有模型都交给 ACU。WorkBuddy 管理附加模型列表；关闭时按内核预览移除其管理的附加项，可能包括其他来源，不宣称只移除 ACU。失败留在弹窗，不退出应用。
- 模型清单：四个 Agent 用同一张连续清单表达 `角色 → 模型`、`＋新增模型`、`原生保留`，默认模型直接在箭头行切换。不再分多个说明区块。详情支持搜索，来源标识与候选点击具体行才展开；尚未接入显示“可添加”，未知原生信息不填解释性占位。模型显示开关、批量显示、排序和当前模型保护直接复用内核原组件。
- Codex 边界：固定内核在 ChatGPT 登录模式下使用统一本地入口，再区分原来源与自定义模型；不是每个模型分别绕过网关。API 接入 / 账号用尽回退的行为不同，不承诺原生模型一直可用。这个结论不泛指 ChatGPT 桌面应用。
- 推荐预览：模型来自内核的 Agent 选项与当前 Key 的交集；WorkBuddy 使用来源模型并说明实际列表遵循已有开关。图片与搜索读取内核现有共享配置并显示具体来源、关闭、缺失或读取失败；不把“已有配置”当成“已验证可用”。默认由 Magpie Library 添加识图、生图及可配置的搜索 MCP，可取消。共享模型保持用户显式选择；首次推荐当前 Key 下的 GPT-6.1 Sol 和 Sunburst。工作台「图片与搜索」可随时更换。
- Agent 日常管理：展开原版 Agent 行，切换模型 / 推理强度、接入或断开；可看到重启提醒。沿用原版选项、写入、还原预览与确认，不另写备份 / 还原逻辑，也不停止其他 Agent 或全局网关。WorkBuddy 等保留应用内选模，原版折叠列表可展开其余 Agent。
- 用量 / 路由：直接读取内核原生统计、图表和请求轨迹；可能包含本机会话日志和其他来源。费用是估算，不代表 ACU 账单或节省金额；不推断 acu-auto 云端最终模型。
- ACU 连接：已有 Key 不强制登录；可网页登录 / 注册并选择账号 Key，也可直接粘贴 Key。官网撤销 Key 每 30 秒检查一次，配置前再次检查，不发收费推理请求。
- 账号登录：在独立原生窗口打开 `console.acucompute.com`，保留网站安全验证；登录后自动返回并连接第一个可用 Key，窗口隐藏后负责后台续期。首页可切换 Key，未自动返回时可点击窗口底部“立即连接”。不收集密码。
- 账号登录：新登录固定使用 `acucompute.com`（实际安全登录页为 `console.acucompute.com`），不再让用户选择站点；首页 Key 管理保持内联。
- 准备模型：只读取可用模型列表，复用内核保存 Chat / Responses / Messages 三种入口；不会自动发收费测试请求，也不会在连接账号时直接修改 Agent 文件。
- Agent 配置：由 Runtime 按 Agent 打开原生模型配置，并直达图片、联网搜索及 MCP 工具模块；选择、配置、还原预览和统计仍由内核原生组件完成。
- 设置：仅保留系统主题、托盘行为和退出入口。第三方许可文件随安装保存在本机，不在用户界面展示。
- 外观：ACU 黑 / 琥珀黄 / 白；外壳、弹窗与内核模块实时跟随系统明暗，不改内核保存的主题。

Windows 已验证真实 acu.best 会话回传、账号 Key 读取、默认 Key 入核；另有隔离原生测试验证 Key 切换。未完成：console.acucompute.com 真实登录、ACU 云端推荐配置自动导入、CC Switch 检测、节省 / 纠偏数据、服务端控制更新、开机启动、签名 / 卸载、Mac 完整业务验收、异步视频。一键接入使用选定 ACU 来源的主模型；WorkBuddy 保留全部可选模型与应用内选择，不主动关闭其他来源。不等于已验证全部多模态能力。

账号访问令牌只保存在 Go 内存中，到期前两分钟通过隐藏的官网窗口自动续期；断网、限流和网站验证故障退避重试，不因短期令牌到期要求重新登录，也不重新选择或覆盖模型 Key。只有续期会话被确认失效、撤销或切换时才要求重新登录。续期 Cookie 仍由系统 WebView 管理，不读取或复制到 Go。退出应用后需再次点击登录，可复用 WebView 的官网会话；本轮不实现启动时自动恢复账号。已保存的模型 Key 仍在内核中，无需重复连接。原生消息校验官方来源、窗口与随机状态，不经 URL、日志或跨站本机请求；刷新还校验账号与服务端会话一致。账号列表最多读取前 100 个 Key；无可用 Key 时保留手填兜底。

## 构建与测试

需要 Go 1.26 及以上。Windows 构建包含离线内核的开发安装包：

```powershell
.\scripts\build-windows.ps1
```

macOS 原生应用、DMG 与 ZIP（在对应架构 Mac 上运行，需要 Xcode 命令行工具）：

```sh
bash scripts/build-macos.sh
```

GitHub Actions 在推送 `v版本号` 标签后构建 Windows x64、macOS arm64 / amd64，全部检查通过再发布含 SHA-256 清单的 Release。

只构建 Windows 原生应用：

```powershell
go build -tags desktop,production -ldflags '-H windowsgui' -o 'bin/ACU Runtime.exe' .
.\bin\ACU Runtime.exe
```

```sh
go test ./...
go test -tags desktop,production ./...
go vet -tags desktop,production ./...
node --test tests/*.test.cjs
```

Agent 路由界面集成测试（需工作区 `.build` 中已有 Playwright、Chrome 和缓存的原版内核界面资源；全部写入均指向隔离 API，不修改真实 Agent）：

```powershell
go build -o '../.build/acu-runtime-routing-web.exe' .
node scripts/test-agent-routing.cjs
```

Windows 隔离原生登录 / 续期测试（需工作区 `.build` 中已有 Playwright，先正常退出运行中的 Runtime）：

```powershell
go test -c -tags desktop,production -o '../.build/acu-runtime-native-login-smoke.exe'
node scripts/test-native-login.cjs
```

使用临时 WebView profile 和模拟账号验证 5 秒访问令牌到期、503 重试、后台续期及 Key 保持；不读取真实登录 Cookie、不写真实 Agent 配置。

不带 desktop 标签的构建只用于开发网页调试，不自动下载 / 启动内核。Mac 原生构建须在 Mac 上准备 Wails 所需的 Xcode 工具链，不能用 Windows 的服务交叉编译冒充。

## 开发约定

- `ACU_KERNEL_PATH` 可指定已有可执行文件；开发者使用 `--kernel-url` 连接已有本机管理接口，管理 Key 通过 `ACU_KERNEL_KEY`，不进入页面。不要把模型网关端口当管理端口。
- 管理服务只监听回环 IP；写入校验本机会话、Host / Origin / FetchSite。代理支持 SSE，前端不接触上游鉴权 Cookie。
- 原版 HTML / JS / CSS 由本地代理嵌入；本版本针对 Magpie `v0.1.3150` 做 API 与 Agent 模型目录兼容检查，不承诺任意版本 DOM 都兼容。升级内核须通过固定版本与校验清单发布，不直接跟随 latest。
- 安装、主题与嵌入测试使用隔离目录 / API，不发收费模型请求、不替用户应用推荐配置。配置入口不等于所有模型已经支持图片、PDF、搜索或视频。

需求与分工见上层 `ACU-Runtime-开发文档-v1.19.md`。开源许可见安装目录中的 `THIRD-PARTY-NOTICES.txt`。


应用更新网络：GitHub 不可用时自动尝试公开代理 gh-proxy.com，下载还可备用 ghfast.top；无需用户配置线路，保留 SHA-256 与随包内核校验。

## 图片与搜索一键配置

「推荐配置」默认勾选「同时配置图片与搜索工具」。Runtime 仅调用 Magpie 的 `/api/settings`、`/api/library/servers/save`，由内核处理 Codex、Claude Code、Hermes、WorkBuddy 的配置格式、备份与分配；不另建 MCP 服务或路由器。主界面和设置中的「图片与搜索」可更换识图 / 生图模型；模型设置由 Magpie 共享。工具目录保留完整的工具开关、模型来源与路由调整能力。

首次识图优先推荐当前 ACU Key 提供的 `gpt-6.1-sol`，生图推荐 `gpt-image-2.5-sunburst-2026-09-08`（或其 Sunburst 别名）。已有选择及 `off` 保留。没有推荐候选时提示用户手选，不随意改用其他生图模型。配置状态不代表收费推理已验证成功；Agent 自己决定工具调用，不保证原生能力与 MCP 的固定优先级。

尚未配置搜索后端时，自动分配 [DuckDuckGo MCP Server](https://github.com/nickclyde/duckduckgo-mcp-server)，MIT 许可、无需付费 Key，固定 `duckduckgo-mcp-server[browser]==0.7.0`。搜索与网页提取由该开源项目实现。已有运行环境直接复用；没有环境时 Runtime 优先使用安装包附带并校验 SHA-256 的 uv 0.13.7；开发版本缺少包内环境时自动从 Astral 官方 Release 下载固定版本并校验，在 ACURuntime 私有工具目录解压，调用其包管理器自动准备 Python 3.12 / MCP 依赖后再分配工具。无需用户选择、管理员权限、修改系统 PATH 或手动安装。正式安装包不需要临时去 GitHub 获取运行环境；首次仍需联网下载 MCP / Python 依赖，可能数分钟；失败明确显示可重试原因。
免费搜索受 DuckDuckGo 可达性和限流影响，搜索内容直接发给 DuckDuckGo，不使用或发送 ACU Key。未来 ACU SearXNG 上线后，在「配置 ACU 搜索服务」填写官方 HTTPS 基址；Runtime 只向官方域名配置当前 ACU Key，由内核执行 Bearer 认证。留空保留原配置，停用 / 移除旧来源在工具目录及内核搜索设置中完成；不会自动删除其他 MCP。

新增工具需重新打开 Agent，WorkBuddy 首次仍需信任。PDF 文件读取继续由 Agent 文件工具处理，扫描件需 OCR / 转换，HTTP 图片结果应及时保存；这些边界不因添加 MCP 自动消失。


### 0.1.38 简化日常配置

工作台「一键配置」默认只需选择应用，首次接入使用推荐模型并自动准备图片与搜索工具。调整模型和范围放在折叠项中；已接入应用默认保留主模型与可见性。想替换时须显式勾选。部分失败只重试未完成应用或工具。

「图片与搜索」用状态与按钮表达应用配置；保存模型设置通过独立模式只修改用户选中的共享模型字段，不安装依赖、不读写应用工具配置。重新配置工具保留显式共享模型和内核工具选项。搜索服务地址、模型来源、工具权限和分配在「高级设置」中直达内核相应模块。返回后重新读取内核当前设置。自定义工具选项以「自定义设置」提示；配置完成与实际推理连通性不混用。


### 图片与搜索管理页

常用模型、应用工具和高级设置使用独立区域；保存按钮只在模型表单内。高级设置合并为「模型与搜索设置」和「工具与应用分配」两个入口。返回工作台或进入高级设置时，只有实际模型修改未保存才弹出保存 / 放弃 / 继续编辑提示；保存失败保留修改并停留原页。从内核返回会重新读取当前设置。


### 通用工具统一检查

图片与搜索页面的「通用工具」仅提供一个「一键检查并补齐」按钮，按应用调用内核完成配置同步，保留已有模型和工具自定义选项。各应用正常时只显示接入状态，失败时才出现错误和单独重试。此操作不重置模型推荐、不发送付费推理请求，也不将配置写入成功等同于业务调用已验证。


### 0.1.42 验收修正

一键配置优先复用 Magpie 内置的免 Key You.com 搜索（其免费额度和可达性由服务方决定），无需额外 Python/MCP 依赖。已有搜索来源及显式停用保留；现有 DuckDuckGo MCP 不删除，仍可在内核中调整。搜索可以使用应用原生服务或内核来源，不强制经过 ACU 上游。

ACU 的 `acu-auto` 目录尚未报告上下文时，通过 Magpie 官方 `provider set <id> context.acu-auto=922000` 补齐；其他模型沿用内核目录报告的各自窗口，显式用户覆盖保留。已确认修复后本机 Hermes 能启动并完成真实 ACU 请求；这不等于已发送 92 万 token 做压力验证。

WorkBuddy 桌面端 GPT-6.1 Sol 已实际调用官方 `workbuddy-pdf` 提取正文、转换页面并识图核对。组件发现偶有退出码 127，同一桌面新任务可成功，后续任务也出现过复发；不将单次成功视为组件稳定性已修复。Runtime 不另外安装 PDF 解析器，也不修改用户的应用权限。禁止本地命令的任务指令会限制其正常文件处理，正常只读解析可以使用现有工具。


推荐工具同时通过 Magpie 技能库分发 `acu-image-tools`：普通生图请求优先发现并调用已配置的图片 MCP，未指定模型时沿用共享生图设置，不静默替换成消耗应用积分的原生 ImageGen。仅为调用指引，不能强制客户端工具选择；用户明确指定原生服务时尊重选择。工具可用性、首次信任与客户端技能加载仍需实际验收。已有同名用户技能和用户修改不覆盖。
