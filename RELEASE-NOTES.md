ACU Runtime 0.1.40：Windows 与 macOS 原生桌面发布。

优先复用已开启的 Magpie 本机管理服务，未开启时才启动必要的控制进程并由内核复用已有推理网关。退出只结束 Runtime 自己启动的进程。修复 Magpie 返回已有服务地址后正常退出被误判启动失败的问题。连接前通过内核命令关闭自动更新，不增加用户选项；用户手动升级不会被强制降级。

通用工具改为统一「一键检查并补齐」：各应用只显示接入状态，不再常驻逐应用重新配置按钮。检查保留已有共享模型和工具选项；某个应用写入失败时在该应用显示原因与重试，不重做已完成应用。

「图片与搜索」改为独立管理页：常用模型局部保存、应用工具独立操作、高级设置独立跳转。模型与搜索合并为同一个内核入口，另保留工具分配入口。离开页面仅在模型有未保存修改时提示保存或放弃；保存失败保留修改，内核返回后重新读取设置。

简化一键配置与图片搜索管理：应用直接显示「已配置 / 待补齐」并提供明确配置按钮；保存模型只修改共享模型，不重写工具。搜索服务、模型来源与工具权限进入内核高级设置。重复一键配置默认保留已接入应用的主模型、模型可见性与工具自定义选项，部分失败可单独重试。

修复更新确认弹窗超出窗口的问题：长说明在正文区域滚动，标题、关闭和更新按钮始终保持可访问。

新增「图片与搜索」管理：推荐配置默认调用 Magpie 内核批量分配识图、生图、搜索 MCP；保留已有模型与 MCP 分配，随时修改共享能力模型。没有搜索后端时自动准备免费 DuckDuckGo MCP（固定 0.7.0，无需 Key）。安装包附带经过 SHA-256 校验的官方固定运行环境，缺少环境时自动准备，放在 Runtime 私有目录，完成依赖准备后分配，无需用户理解或安装 uvx。ACU SearXNG 地址仍需服务上线后填写，PDF / OCR 继续依赖应用文件工具。

包含近期 Agent 工作台、模型清单和可见性管理、推荐配置、账号登录续期、主题与下拉框兼容修复。接入状态统一显示“已接入 ACU”，简化模型行的展开箭头与当前使用状态。内置固定并校验 SHA-256 的 Magpie v0.1.3150 备用内核。

| 设备 | 下载文件 | 安装方式 |
| --- | --- | --- |
| Windows 10/11 x64 | `ACU-Runtime-0.1.40-windows-amd64-Setup.exe` | 退出旧版后双击安装，开始菜单打开 ACU Runtime |
| Mac Apple Silicon（M1/M2/M3/M4 等） | `ACU-Runtime-0.1.40-macos-arm64.dmg` | 打开 DMG，将 ACU Runtime 拖入 Applications |
| Mac Intel | `ACU-Runtime-0.1.40-macos-amd64.dmg` | 打开 DMG，将 ACU Runtime 拖入 Applications |

macOS 需要 12 或更新版本；另附 `.app.zip` 作为备用分发格式。安装后打开应用，登录 ACU 或填写已有 Key，选择需要接入的 Agent。首次应用配置前可以预览变更。Windows 需要 Microsoft Edge WebView2 Runtime，Windows 11 通常已内置；缺失时安装 Microsoft 官方 Evergreen WebView2 Runtime。

本版未使用 Windows 代码签名或 Apple Developer ID 公证。Windows 可能显示 SmartScreen 提醒；macOS 首次打开可能需要在“系统设置 → 隐私与安全性”中对 ACU Runtime 选择“仍要打开”并确认。仅在确认文件来自此仓库后执行此操作，无需关闭系统安全保护。macOS 包使用本机 ad-hoc 签名，不能等同于 Apple 公证。

Windows x64 支持应用内更新；macOS 更新请退出应用后用新版替换 Applications 内的旧版。用户 Key 和 Agent 配置保留。Runtime 连接前使用 Magpie 原生命令关闭自动检查、自动下载和自动重启更新；独立安装不被覆盖或降级，用户手动升级保留。Windows ARM64 不在本次安装包支持范围内。

发布流水线在 Windows x64、macOS Apple Silicon 和 Intel runner 上执行 Go/原生桌面测试、vet 与前端测试。macOS 包另验原生桌面启动、回环界面、应用结构与签名。真实账号登录和各 Agent 的完整业务验收仍需对应设备验证。所有安装包摘要见 `SHA256SUMS.txt`。
