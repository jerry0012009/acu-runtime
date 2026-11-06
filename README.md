# ACU Runtime · 桌面基础版 0.1.14

Wails 原生应用 + 独立内核服务，不需要浏览器。关闭窗口驻留托盘，再次启动唤醒同一实例。

## 安装与使用

Windows 安装包：双击 `dist/windows-amd64/ACU Runtime Setup.exe`，安装到当前用户目录并自动打开，开始菜单有应用入口，无需管理员权限。当前尚未签名，也未提供注册卸载器，不作为正式分发验收。

安装包自带 Magpie `v0.1.3150` 备用内核。Runtime 优先连接本机已有 Magpie；安装或更新 ACU Runtime 时都会刷新安装目录内的备用内核，既有 Agent 配置保留。找不到独立 Magpie 时使用此备用内核；单独运行未附带内核的应用时，首次启动会从官方 Release 下载并校验 SHA-256。内核包支持 Windows / macOS 的 amd64、arm64；macOS 的实机验证需在对应设备完成。

老用户更新：先从托盘退出 Runtime，再从 [GitHub Releases](https://github.com/jerry0012009/acu-runtime/releases) 下载并运行新版安装包，覆盖安装即可同时更新 Runtime 与它管理的备用内核。独立安装的 Magpie 仍由 Magpie 自己更新；Runtime 不覆盖该安装。当前未实现后台自动更新，不能仅靠重启旧 Runtime 获得新版。每个 Runtime 发行版固定经过检查的内核版本，详细验证范围见 [COMPATIBILITY.md](COMPATIBILITY.md)。

打开后自动连接，无需找路径、选择管理端口或确认启动。失败才显示原因及“重试”。只收起窗口不会停服务；从托盘退出会停止本次启动的管理进程，不停止原有内核。内核启动会执行其正常同步，可能在原网关未运行时接管网关。

## 已接入

- 托盘快捷面板：单击右下角 ACU 图标弹出原生无边框窗口，复用内核的 Agent / 用量 / 路由三页；点外部收起，右键菜单打开主窗口或退出。面板内“打开主窗口”进入对应详情，窗口操作不会发送给原有内核 GUI。
- 工作台：新用户同时看到三步引导和 Agent 工作台；已有连接直接显示完成状态。登录、Key 选择和连接管理在同一页，账号与手动 Key 管理按需展开。
- 主界面：导航只保留工作台、用量、请求记录、设置。Agent 状态按原生配置、ACU 路由、应用内选模和重启要求表达；单个 Agent 接入先确认再写入。
- 单 Agent 接入：卡片直接打开该应用的推荐配置，兼容模型中优先选 acu-auto；查看默认路由、原有与 ACU 模型、能力影响后确认。只写入所选 Agent，不重复选择应用，也不展示辅助模型同步开关。WorkBuddy 不设默认模型，保留任务内选择。顶部入口命名为“推荐配置”，按推荐值完成首次接入；需要细调时再进入“高级配置”。
- 接入开关：卡片上方的大开关管理整个 Agent 的本地接入，不代表所有模型都交给 ACU。WorkBuddy 管理附加模型列表；关闭时按内核预览移除其管理的附加项，可能包括其他来源，不宣称只移除 ACU。失败留在弹窗，不退出应用。
- 模型清单：四个 Agent 用同一张连续清单表达 `角色 → 模型`、`＋新增模型`、`原生保留`，默认模型直接在箭头行切换。不再分多个说明区块。详情支持搜索，来源标识与候选点击具体行才展开；尚未接入显示“可添加”，未知原生信息不填解释性占位。模型显示开关、批量显示、排序和当前模型保护直接复用内核原组件。
- Codex 边界：固定内核在 ChatGPT 登录模式下使用统一本地入口，再区分原来源与自定义模型；不是每个模型分别绕过网关。API 接入 / 账号用尽回退的行为不同，不承诺原生模型一直可用。这个结论不泛指 ChatGPT 桌面应用。
- 推荐预览：模型来自内核的 Agent 选项与当前 Key 的交集；WorkBuddy 使用来源模型并说明实际列表遵循已有开关。图片与搜索读取内核现有共享配置并显示具体来源、关闭、缺失或读取失败；不把“已有配置”当成“已验证可用”。本次不自动添加 MCP，不承诺路由后所有多模态能力都可用。
- Agent 日常管理：展开原版 Agent 行，切换模型 / 推理强度、接入或断开；可看到重启提醒。沿用原版选项、写入、还原预览与确认，不另写备份 / 还原逻辑，也不停止其他 Agent 或全局网关。WorkBuddy 等保留应用内选模，原版折叠列表可展开其余 Agent。
- 用量 / 路由：直接读取内核原生统计、图表和请求轨迹；可能包含本机会话日志和其他来源。费用是估算，不代表 ACU 账单或节省金额；不推断 acu-auto 云端最终模型。
- ACU 连接：已有 Key 不强制登录；可网页登录 / 注册并选择账号 Key，也可直接粘贴 Key。官网撤销 Key 每 30 秒检查一次，配置前再次检查，不发收费推理请求。
- 账号登录：在独立原生窗口打开 `console.acucompute.com`，保留网站安全验证；登录后自动返回并连接第一个可用 Key，窗口隐藏后负责后台续期。首页可切换 Key，未自动返回时可点击窗口底部“立即连接”。不收集密码。
- 账号登录：新登录固定使用 `acucompute.com`（实际安全登录页为 `console.acucompute.com`），不再让用户选择站点；首页 Key 管理保持内联。
- 准备模型：只读取可用模型列表，复用内核保存 Chat / Responses / Messages 三种入口；不会自动发收费测试请求，也不会在连接账号时直接修改 Agent 文件。
- Agent 配置：由 Runtime 按 Agent 打开原生模型配置，并直达图片、联网搜索及 MCP 工具模块；选择、配置、还原预览和统计仍由内核原生组件完成。
- 设置：仅保留系统主题、托盘行为和退出入口。第三方许可文件随安装保存在本机，不在用户界面展示。
- 外观：ACU 黑 / 琥珀黄 / 白；外壳、弹窗与内核模块实时跟随系统明暗，不改内核保存的主题。

Windows 已验证真实 acu.best 会话回传、账号 Key 读取、默认 Key 入核；另有隔离原生测试验证 Key 切换。未完成：console.acucompute.com 真实登录、ACU 推荐配置自动导入、自动能力补齐、CC Switch 检测、节省 / 纠偏数据、服务端控制更新、开机启动、签名 / 卸载、Mac 实机验收、异步视频。一键接入使用选定 ACU 来源的主模型；WorkBuddy 保留全部可选模型与应用内选择，不主动关闭其他来源。不等于已验证全部多模态能力。

账号访问令牌只保存在 Go 内存中，到期前两分钟通过隐藏的官网窗口自动续期；断网、限流和网站验证故障退避重试，不因短期令牌到期要求重新登录，也不重新选择或覆盖模型 Key。只有续期会话被确认失效、撤销或切换时才要求重新登录。续期 Cookie 仍由系统 WebView 管理，不读取或复制到 Go。退出应用后需再次点击登录，可复用 WebView 的官网会话；本轮不实现启动时自动恢复账号。已保存的模型 Key 仍在内核中，无需重复连接。原生消息校验官方来源、窗口与随机状态，不经 URL、日志或跨站本机请求；刷新还校验账号与服务端会话一致。账号列表最多读取前 100 个 Key；无可用 Key 时保留手填兜底。

## 构建与测试

需要 Go 1.26 及以上。Windows 构建包含离线内核的开发安装包：

```powershell
.\scripts\build-windows.ps1
```

只构建原生应用：

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

