# ACU Runtime 0.1.48

轻量安装器、一键安装 ChatGPT / WorkBuddy、启动更新提醒及首页更新入口。

- 普通 Windows 安装器约 62 MB，包含配套 Magpie v0.1.3852，不捆绑第三方客户端。已安装应用直接复用，首次只下载用户选择的应用。
- Windows ChatGPT 首装与手动更新优先使用 AgentsMirror 的滚动 latest 国内可访问线路，备用 OpenAI 官方完整 MSIX，无需 Microsoft Store。在线安装刷新版本清单，再检查缓存；对照官方更新清单，镜像滞后时切换官方线路。官方清单暂不可达时仅接受七天内的镜像清单，不能保证该情况下绝对最新。下载验证大小、SHA-256、包版本、官方签名、发布者、身份和架构，支持断点续传。
- WorkBuddy 使用腾讯国内官方 CDN，安装前查询官方更新 API 获取最新架构包，校验官方摘要及厂商签名，保留客户端内更新。其首次登录仍按官方客户端要求完成。
- 没有 ChatGPT 账号时复用 Magpie 的 API 模式，已有账号保留；仍需可用 ACU Key，云端账号功能仍需官方登录。
- Windows 启动后检查一次正式版本；发现新版弹出确认，确认后更新 Runtime 与配套内核并重启。安装 Agent 时延后提醒。已有版本不能凭本次发布自动获得它尚未实现的启动提醒，可从设置检查更新。
- 首页提供低强调更新栏和“全部更新”；Magpie 确认有新版本时显示 Agent 更新。Codex CLI 与 ChatGPT 桌面更新分别标注，WorkBuddy 继续使用客户端内更新。
- 适配新内核共享选项索引；保留已有模型、Key、MCP 与独立 Magpie 安装。可选全量离线 ZIP 与普通安装器分开。

Windows x64 支持应用内更新。macOS 12+ 提供 Apple Silicon / Intel DMG 和 app.zip，Runtime 更新仍需替换应用；ChatGPT 自身系统要求以官方安装包为准。系统 UAC、工具信任与组织安装策略保持有效。

## 下载

| 设备 | 文件 |
| --- | --- |
| Windows 10/11 x64 | ACU-Runtime-0.1.48-windows-amd64-Setup.exe |
| Mac Apple Silicon | ACU-Runtime-0.1.48-macos-arm64.dmg |
| Mac Intel | ACU-Runtime-0.1.48-macos-amd64.dmg |

本版 Runtime 没有 Windows 代码签名或 Apple Developer ID 公证；macOS 包使用 ad-hoc 签名。安装包摘要见 SHA256SUMS.txt。

## 验证范围

发布流水线验证 Go、原生桌面、前端、三平台打包及配套内核 API；一次性 runner 验证 WorkBuddy 官方安装和 Windows ChatGPT 官方签名包首次安装。无 ChatGPT 账号的配置与桌面所用 app-server 协议另行验收，不能用模型配置测试冒充完整 GUI 交互。具体结果见 COMPATIBILITY.md。

国内镜像是第三方分发服务；不传输 ACU Key 或用户会话。下载包必须通过官方签名验证，线路不可用时可以重试或使用官方来源。ACU 自有服务未部署新的下载代理。
