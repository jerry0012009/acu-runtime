# 用量与费用实现

沿用现有 Runtime / Magpie 分工，不改 ACU frontend、不增加本地计费系统。

- **本机用量**：继续嵌入 Magpie 原页面，保留时间范围和明细查询参数。状态轮询与账户刷新不重建 iframe；费用保持“估算”。顶部展示独立的账户余额摘要。
- **ACU 账户**：通过 Go 内存中的官网登录令牌读取 `/api/user/self`、所选 Key 的 `/api/token/:id`、`/api/log/self/stat` 和 `/api/log/self`。范围为该账户全部 Key、全部设备，支持最近 24 小时 / 7 天 / 30 天，每页 20 条。Key 只代表上次通过账号连接的 Key，手填 Key 不推断账户归属。
- **模型价格**：读取官网 `/api/pricing` 的已发布静态价格，缓存 5 分钟；可手动刷新。最近成功目录保存到桌面缓存目录 `model-prices.json`，重启先展示缓存再更新，超过 30 天或损坏的缓存不使用。采用所选官方站点，首站失败或等待 600 毫秒后尝试另一官方站点，总网络预算 8 秒，重复刷新合并；不携带账号凭证。Windows 复用已有系统代理支持。展示人民币输入、输出、缓存读取单价，支持名称搜索和价格排序；详情展示缓存写入、上下文分档、倍率参考类型和价格版本。`acu-auto` 按实际选中模型的已发布价格计费，不伪造固定价格。

## 金额口径

云端历史汇总使用服务端 `user_charge_cny`，详情优先使用历史记录中的同名精确金额及价格版本。旧详情缺少精确金额时，将服务端已扣除的 `quota` 按 `/api/status` 的 `usd_exchange_rate / quota_per_unit` 换算，并标注“历史额度换算”。不使用当前模型价格重算历史费用，不把充值 `price` 当作汇率。服务端明确金额和零值与缺失数据分别处理。

账户余额和 Key 上限分别展示。请求令牌、Key 明文、日志正文及完整 `other` 不返回浏览器。账号切换或退出后丢弃旧请求结果。账户页在可见时每分钟刷新，失败保留旧值与更新时间；价格失败同样保留最近一次成功目录。本机统计独立可用。

## 重启恢复登录

网页登录成功后仅保存官方站点、用户 ID、服务端会话 ID、所选 Key ID 到系统缓存目录 `ACURuntime/account-session.json`；登录 Cookie 仍由系统 WebView profile 管理，不另存访问令牌、刷新令牌或模型 Key。

启动读取标识后创建隐藏官网窗口，通过现有 `/api/user/auth/refresh` 刷新并校验同一用户、同一会话。恢复只读身份和 Key 清单，不重复选择第一个 Key、不调用 Key 明文接口、不写 Provider 或 Agent 配置。临时网络错误退避重试；确认过期 / 撤销后显示重新登录。账号页可手动登录接管恢复。

“退出账号”清除 Runtime 自动登录记录与内存会话，隐藏续期窗口随后关闭；已保存模型 Key 与 Agent 配置保留。不会撤销用户其他客户端的官网会话。

旧版本没有保存账号标识，升级后需要先登录一次，之后重启自动恢复。

## 验证

```powershell
go test ./...
go test -tags desktop,production ./...
go vet -tags desktop,production ./...
node --test tests/*.test.cjs
node scripts/test-billing.cjs
go test -c -tags desktop,production -o '../.build/acu-runtime-native-login-smoke.exe'
node scripts/test-native-restore.cjs
```

浏览器验证使用隔离接口，覆盖本机统计保留、历史扣费、分页、搜索、分档、错误保留、账号隔离及浅深色布局。原生验证使用独立实例 ID、临时 WebView profile、假账号和假 Key，不触碰真实 Agent 配置。
