# Runtime 0.1.16 / Magpie 0.1.3150

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

当前应用内安装包只支持 Windows x64。Windows ARM64 与 macOS 还没有对应 Runtime 安装包；会报告平台不可用而不进行下载。后续发布这些平台的签名安装器后，才能开放一键安装。独立 Magpie 不受 Runtime 更新器管理。

更新网络：优先使用显式 HTTP(S)_PROXY/NO_PROXY 环境设置，否则读取 Windows 手动系统代理。PAC 自动代理与国内镜像尚未接入。无法访问 GitHub 时保留现有安装，可稍后重试。

## 0.1.16 在线更新验收

- 本机真实 0.1.15 安装经正式 GitHub Release 检查、下载、校验后更新到 0.1.16；旧进程退出，新进程自动启动并返回 ready=true。
- 更新后的 Runtime 可执行文件与发布构建摘要一致，备用内核摘要对应 Magpie 0.1.3150。
- 更新前后 Codex、Claude、Magpie providers/settings 共 4 个关键配置文件 SHA-256 不变。
- 旧版测试进程临时设置 HTTPS_PROXY 以使用本机已有代理；0.1.16 随后在无此临时环境变量的新进程中，通过 Windows 手动系统代理完成检查并报告 current。
- Windows ARM64 桌面和 macOS amd64/arm64 服务端交叉编译通过；不代表其他平台的实机运行验收。
- 0.1.15 已发布但不读取 Windows 手动系统代理，建议新用户安装 0.1.16。国内无 GitHub 网络的用户仍需后续官方镜像；当前未接入公共代理或镜像。
