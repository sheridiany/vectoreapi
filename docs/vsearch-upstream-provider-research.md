# vSearch 上游研究：JustOneAPI 与 TikHub

> 调研日期：2026-08-29
>
> 范围：仅使用 JustOneAPI、TikHub 官方网站、官方文档、官方 GitHub、官方条款和官方 OpenAPI。
>
> 说明：本文不是法律意见；涉及转售、数据授权和跨境合规的结论，必须由供应商书面合同和专业法律意见最终确认。

## 1. 结论先行

1. **[官方事实] TikHub 当前用户后台条款把许可限定为内部业务用途，且禁止未经书面授权转售、再分发或构建竞争产品。**购买余额、RPS 套餐或 Enterprise 资格本身不改变这个许可边界。[TikHub 当前 Terms of Service](https://user.tikhub.io/terms)
2. **[官方事实] JustOneAPI 公共条款要求合法使用、承担数据下游使用责任，但没有公开授予 API 转售、再许可或白标分发权。**[JustOneAPI Terms of Service](https://justoneapi.com/en/terms)
3. **[推断] 现在不应直接购买两个高阶会员并把能力作为付费公共 API 对外开放。**正确顺序是：免费额度/小额充值验证真实接口与账单，再取得两家明确的 reseller / white-label / downstream SaaS 书面授权，最后才开放企业付费路由。
4. **[官方事实] TikHub 更适合做重社交平台的主供应商：公开 OpenAPI 当前包含 1,021 个路径，覆盖抖音、TikTok、小红书、Bilibili、快手、微信、微博、Instagram、YouTube、Twitter/X、Threads、Reddit、LinkedIn、知乎等深层接口。**该数量是 2026-08-29 对官方 OpenAPI 的直接统计。[TikHub OpenAPI](https://api.tikhub.io/openapi.json)
5. **[官方事实] JustOneAPI 的公共 OpenAPI 当前包含 299 个路径，除重叠的社交平台外，还覆盖 1688、淘宝/天猫、京东、Amazon、Temu、Shopee、闲鱼、得物、贝壳、豆瓣电影、IMDb、YOUKU 等。**该数量是 2026-08-29 对官方 OpenAPI 的直接统计。[JustOneAPI OpenAPI](https://docs.justoneapi.com/openapi.json)
6. **[推断] vSearch 不应把两个上游名称相似的接口直接互为 fallback。**只有在请求语义、响应字段、分页、时效性、计费触发和法律权限都经契约测试确认等价后，才能配置主备。
7. **[推断] 当前工程应把 OpenAPI REST 适配器作为生产执行面，MCP 只作为 AI 发现/调用入口或目录参考。**JustOneAPI 官方 MCP 明确采用 `search_endpoints -> get_endpoint_schema -> call_endpoint` 的动态工具流程，适合 Agent 使用，但不是多上游路由的唯一内部抽象。[JustOneAPI MCP tools](https://github.com/justoneapi/justoneapi-mcp/blob/main/TOOLS.md)

## 2. 证据标记

- **[官方事实]**：官方页面、官方文档、官方 GitHub 或官方 OpenAPI 直接陈述/可直接统计。
- **[推断]**：基于官方事实做出的产品、架构或商业判断。
- **[未知]**：公开资料没有给出确定答案，必须在供应商后台或书面合同中确认。

## 3. 一览对比

| 维度 | JustOneAPI | TikHub | 对 vSearch 的影响 |
| --- | --- | --- | --- |
| 公开目录快照 | **[官方事实]** 299 paths：282 GET、17 POST、36 tags。[OpenAPI](https://docs.justoneapi.com/openapi.json) | **[官方事实]** 1,021 paths：797 GET、224 POST、51 tags，版本 V5.3.2。[OpenAPI](https://api.tikhub.io/openapi.json) | **[推断]** 保存 OpenAPI hash/version，变更先审查再发布，不能同步后直接公开。
| 核心优势 | **[官方事实]** 社交 + 电商 + 房产 + 影视等更宽的垂直覆盖。[官方目录](https://docs.justoneapi.com/en/) | **[官方事实]** 16 个平台、1,000+ 实时接口，社交平台接口深度更高。[Pricing](https://tikhub.io/pricing) | **[推断]** TikHub 主做重社交；JustOneAPI 补电商、跨平台和非社交缺口。
| 认证 | **[官方事实]** 每次请求把 `token` 放在 URL query。[Usage](https://docs.justoneapi.com/en/usage) | **[官方事实]** 推荐 `Authorization: Bearer ...`；官方 SDK 只实现 Bearer。[OpenAPI](https://api.tikhub.io/openapi.json)、[SDK auth](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/authentication.md) | **[推断]** JustOneAPI token 必须从 URL、access log、trace、error 和 analytics 中统一脱敏。
| 公开计费 | **[官方事实]** 价格需登录后台逐接口查看，有免费试用，余额不过期。[Usage](https://docs.justoneapi.com/en/usage)、[FAQ](https://justoneapi.com/en/faq) | **[官方事实]** `$0.001-$0.01/request`，按日总量最高 50% 折扣；非 200 不计费。[Pricing](https://tikhub.io/pricing) | **[推断]** 成本不能写死；需要定价快照、币种、有效期和账单对账。
| 默认限速 | **[官方事实]** 无通用限速，少数接口有独立限制；可按账号×接口配置日成功额度。[Usage](https://docs.justoneapi.com/en/usage) | **[官方事实]** 默认 10 RPS，按 API path 独立计算；可购买更高 RPS。[Pricing](https://tikhub.io/pricing) | **[推断]** 限流键至少为 `provider + account + path`，不能只按供应商账号限流。
| 超时/重试 | **[官方事实]** 建议 120 秒、至少 60 秒；官方 Python transport 每次只发一次，没有内置 retry loop。[Usage](https://docs.justoneapi.com/en/usage)、[transport](https://github.com/justoneapi/justoneapi-python/blob/main/justoneapi/_transport.py) | **[官方事实]** OpenAPI 建议 30–60 秒、最多重试 3 次；官方 SDK 对网络、超时、429、5xx 自动指数退避。[OpenAPI](https://api.tikhub.io/openapi.json)、[SDK retries](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/retries.md) | **[推断]** 不能统一套一个 retry policy；适配器按上游计费语义和响应状态决定。
| Webhook | **[官方事实]** 当前官方 OpenAPI 没有顶层 `webhooks` 对象。[OpenAPI](https://docs.justoneapi.com/openapi.json) | **[官方事实]** 当前官方 OpenAPI 没有顶层 `webhooks` 对象。[OpenAPI](https://api.tikhub.io/openapi.json) | **[未知]** 个别异步接口是否有私有 callback；默认按提交+轮询设计，不能承诺统一 webhook。
| 转售/再许可 | **[未知]** 公共条款没有明确授权。[Terms](https://justoneapi.com/en/terms) | **[官方事实]** 当前后台条款明确限制内部用途并禁止未经书面授权转售/再分发。[Terms](https://user.tikhub.io/terms) | **[推断]** 未取得两家书面授权前，只能做内部/管理员试点，不能公开付费售卖。

## 4. JustOneAPI 深度研究

### 4.1 套餐、余额和计费

- **[官方事实]** 注册后有有限免费调用额度，耗尽后充值；每个接口的具体价格只能登录后台查看。[Usage](https://docs.justoneapi.com/en/usage)
- **[官方事实]** 账号余额不会过期。[FAQ](https://justoneapi.com/en/faq)
- **[官方事实]** 业务响应 `code=0` 才被列为成功且计费；`100/301/302/303/400/500/600/601/602` 在公开表格中均列为不计费。[Usage](https://docs.justoneapi.com/en/usage)
- **[官方事实]** `601` 表示共享账号余额不足，`602` 表示某 token 的累计预算上限已到；多个 token 仍共享账号余额。[Usage](https://docs.justoneapi.com/en/usage)
- **[未知]** 公共资料没有会员等级、批量折扣、Enterprise 门槛、发票、币种、退款、SLA 或专线价格表；不能把“充值”理解为会员或转售授权。[FAQ](https://justoneapi.com/en/faq)
- **[推断]** 采购前必须从已登录后台导出计划使用接口的实时价格、币种和计费说明，并将其作为 vSearch 的带时间戳价格快照。

### 4.2 速率、额度和并发

- **[官方事实]** 平台没有通用速率限制，但高流量接口可能有按分钟/小时的单独限制，具体以接口文档为准。[Usage](https://docs.justoneapi.com/en/usage)
- **[官方事实]** 管理员可按用户×接口配置每日成功请求额度；账号下所有 token 合并统计；时区为 `Asia/Shanghai`；达到额度返回业务码 `303` 与 HTTP 429，且不计费。[Usage](https://docs.justoneapi.com/en/usage)
- **[未知]** 并发连接数、每账号总并发、突发桶大小、429 的 `Retry-After`、配额提升价格和审批周期未公开。
- **[推断]** 即使官方说“无通用限速”，vSearch 仍应为每个绑定配置保守的本地 token bucket 和并发上限，避免某接口的隐性限制拖垮整个账号。

### 4.3 认证与安全

- **[官方事实]** REST 生产 base URL 为 `https://api.justoneapi.com`，认证 token 位于 URL query。[Usage](https://docs.justoneapi.com/en/usage)
- **[官方事实]** 官方 Python SDK 也把 token 合并到 query 参数，并默认跟随重定向。[transport](https://github.com/justoneapi/justoneapi-python/blob/main/justoneapi/_transport.py)
- **[推断]** vSearch 的生产适配器不应继承 SDK 的跨主机自动重定向行为；只允许同源受控重定向，否则 query token 可能被发往错误主机。
- **[推断]** URL 中的 `token` 必须在反向代理日志、APM span、异常报告、审计详情和用户可见错误中全部擦除；禁止把完整上游 URL写入业务日志。

### 4.4 接口目录与请求响应

- **[官方事实]** 2026-08-29 的官方 OpenAPI 包含 299 个路径、299 个 operation、36 个 tag，其中 282 个 GET、17 个 POST。[OpenAPI](https://docs.justoneapi.com/openapi.json)
- **[官方事实]** 目录覆盖微信、微信视频号、知乎、YouTube、小红书/蒲公英、微博、Twitter/X、头条、TikTok/TikTok Shop、Reddit、LinkedIn、Instagram、抖音/星图/电商、Bilibili、快手，以及淘宝天猫、京东、1688、Temu、Shopee、Amazon、闲鱼、得物、贝壳、豆瓣电影、IMDb、YOUKU 等。[官方目录](https://docs.justoneapi.com/en/)
- **[官方事实]** 标准成功判断是响应体 `code == 0`；官方 SDK 暴露 `success/code/message/data/raw_json`，而非仅依赖 HTTP status。[SDK README](https://github.com/justoneapi/justoneapi-python/blob/main/README.en.md)
- **[官方事实]** 单个 endpoint schema 可能包含请求参数、响应模型和版本信息；例如跨平台搜索有独立官方 schema。[Cross-platform search](https://docs.justoneapi.com/en/api/social-media/cross-platform-search-v1)、[schema](https://docs.justoneapi.com/openapi/social-media/cross-platform-search-v1-en.json)
- **[推断]** 同一平台的 `V1/V2/V3...` 不能按数字大小自动升级；JustOneAPI 条款明确说版本号不保证更优或永久可用。[Terms](https://justoneapi.com/en/terms)

### 4.5 错误、重试和不确定结果

| 情况 | 官方事实 | vSearch 建议（推断） |
| --- | --- | --- |
| `code=301` | 采集失败，官方写明可重试且不计费。[Usage](https://docs.justoneapi.com/en/usage) | 只做有限次数、带 jitter 的重试，并记录每次上游 request ID。
| `code=302` / HTTP 429 | 接口限速且不计费。[Usage](https://docs.justoneapi.com/en/usage) | 若无 `Retry-After`，按绑定的保守退避；不可立刻并发重放。
| `code=303` / HTTP 429 | 当日额度已满，不计费。[Usage](https://docs.justoneapi.com/en/usage) | 当天熔断该账号×接口；只有等额度窗口重置或切换有授权的等价账号。
| `code=400/600/601/602` | 参数、权限、余额或 token 预算错误，均不计费。[Usage](https://docs.justoneapi.com/en/usage) | 不自动重试；分别映射为标准 `invalid_request/forbidden/upstream_balance_exhausted/upstream_budget_exhausted`。
| 连接已发出但本地超时 | 官方提醒短超时可能导致意外错误或重复收费。[Usage](https://docs.justoneapi.com/en/usage) | 不应跨供应商自动重放；返回 `result_unknown`，用账单/请求日志对账后再结算。

- **[官方事实]** 官方 Python transport 没有自动 retry loop；一次调用只发出一次 GET，然后解析业务码。[transport](https://github.com/justoneapi/justoneapi-python/blob/main/justoneapi/_transport.py)
- **[推断]** “官方说 301 可重试”不等于所有网络错误都安全重试；无法确认请求是否被上游执行时，重复调用可能产生第二次真实采集和费用。

### 4.6 MCP 与 Webhook

- **[官方事实]** 官方 MCP 提供目录搜索、schema 获取、接口调用、余额和用量查询等工具，并推荐先搜索、再取 schema、最后调用。[MCP README](https://github.com/justoneapi/justoneapi-mcp)、[TOOLS](https://github.com/justoneapi/justoneapi-mcp/blob/main/TOOLS.md)
- **[推断]** MCP 应保留为外部 Agent 接入面；内部目录同步和生产执行直接消费官方 OpenAPI/REST，才能准确保存 method/path/schema hash/计费规则。
- **[官方事实]** 当前官方 OpenAPI 没有顶层 webhook 定义。[OpenAPI](https://docs.justoneapi.com/openapi.json)
- **[未知]** 某些异步任务是否可向企业客户开放 callback，需要按 endpoint 和合同确认。

### 4.7 数据授权与商用边界

- **[官方事实]** 条款要求遵守接口/账号限制，不得绕过认证、额度或速率控制；客户必须确保采集、存储、分析及后续使用有合法依据。[Terms](https://justoneapi.com/en/terms)
- **[官方事实]** 第三方平台的条款和隐私规则独立适用；字段、可用性和上游行为可能变化；条款不转移服务或第三方数据所有权。[Terms](https://justoneapi.com/en/terms)
- **[未知]** 公共条款没有明确授予：API 转售、再许可、白标网关、把原始数据返回给企业客户、长期保存原始数据、训练模型或跨境传输的权利。[Terms](https://justoneapi.com/en/terms)
- **[官方事实]** 公共隐私政策明确主要描述公开网站，账号、账单和 API 请求处理可能涉及未在该政策覆盖的其他系统。[Privacy](https://justoneapi.com/en/privacy)
- **[未知]** API 请求参数/响应 payload 的保存周期、处理地点、DPA、子处理商和删除机制未在公共资料中说明。

## 5. TikHub 深度研究

### 5.1 套餐、余额和计费

- **[官方事实]** 新账号有约 `$0.05` 免费额度、约 50 次请求，无需信用卡；全部 16 个平台可访问，但部分 endpoint 仅接受付费余额。[Pricing](https://tikhub.io/pricing)、[Getting Started](https://tikhub.io/getting-started)
- **[官方事实]** Pay-as-you-go 的公开范围是 `$0.001-$0.01/request`，实际价格按 endpoint；日总量阶梯折扣为 0–1,000 基准价，1,000–5,000 减 10%，5,000–10,000 减 20%，10,000–20,000 减 30%，20,000–30,000 减 40%，30,000+ 减 50%。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** 折扣按账号全部 endpoint 的每日总请求量计算；非 200 请求不计费。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** 相同参数的重复请求仍返回实时数据并独立计费；成功响应的 `cache_url` 只用于 24 小时免费调试/分享，不是低价重复获取方式。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** Enterprise 门槛为一次性充值 `$3,000` 或累计 `$4,500`；权益包括支付手续费减免、赠送余额、优先定制接口、专属支持和可用的私有部署；超过 450 万总请求可咨询特别价格。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** 数据集是独立商品，和实时 API 分开计价。[Pricing](https://tikhub.io/pricing)
- **[推断]** Enterprise 是商业计费/支持等级，不是 reseller license。没有书面授权时，即使达到 Enterprise 也不能把 TikHub API 包装后转售。

### 5.2 RPS、并发和区域域名

| 等级 | 官方 RPS | 月费 |
| --- | ---: | ---: |
| Level 1 | 10 | Free |
| Level 2 | 20 | `$5/month` |
| Level 3 | 30 | `$10/month` |
| Level 4 | 40 | `$20/month` |
| Level 5 | 50 | `$30/month` |
| Level 6 | 60 | `$35/month` |
| Level 7 | 70 | `$40/month` |
| Level 8 | 80 | `$45/month` |
| Level 9 | 90 | `$50/month` |
| Level 10 | 100 | `$55/month` |
| Enterprise | 100+ | 联系供应商 |

上表是 2026-08-29 对官方 Pricing 页的 **RPS plans** 数据读取；RPS 计划与用量折扣相互独立。[Pricing](https://tikhub.io/pricing)

- **[官方事实]** 默认 10 RPS 按 endpoint/path 计算，不是按账号总量；调用不同路径时不会竞争同一个默认额度。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** 中国大陆调用方使用 `https://api.tikhub.dev`，大陆外使用 `https://api.tikhub.io`；接口路径和参数相同。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[推断]** 选择域名应依据 gate-relay 服务端出口区域，而不是管理员浏览器所在地。
- **[未知]** 文档公开了 RPS，但没有明确每 endpoint 并发连接数、排队长度、burst、HTTP/2 stream 上限和 RPS 套餐的生效/降级时间。

### 5.3 认证与安全

- **[官方事实]** 推荐使用 `Authorization: Bearer <token>`；Cookie 方式仅为无法使用 header 时的非推荐备选。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[官方事实]** 官方 SDK 只支持 Bearer token，不支持 query key、Basic Auth 或 OAuth。[SDK authentication](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/authentication.md)
- **[官方事实]** OpenAPI 说明每位用户最多创建 20 个 API key，key 创建后只显示一次。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[推断]** 为生产、预发、目录同步和对账分别创建 token，并在供应商可用能力范围内设置独立预算/权限；不允许前端直接持有上游 token。

### 5.4 接口目录与请求响应

- **[官方事实]** 2026-08-29 官方 OpenAPI 版本为 V5.3.2，含 1,021 个路径、1,021 个 operation、51 个 tag，其中 797 GET、224 POST。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[官方事实]** 官方 Python SDK 的生成参考当前写有 1,010 endpoints / 52 resources，与实时 OpenAPI 统计存在漂移。[SDK reference](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/reference.md)、[OpenAPI](https://api.tikhub.io/openapi.json)
- **[推断]** 生产目录以实时 OpenAPI 快照为准，SDK 只作调用参考；每次同步必须记录版本、抓取时间、hash 和 diff。
- **[官方事实]** 平台族包括抖音、TikTok、TikTok Shop、西瓜视频、头条、小红书/蒲公英、Lemon8、Bilibili、快手、皮皮虾、微信、微博、Instagram、YouTube、Twitter/X、Threads、Reddit、LinkedIn、知乎等。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[官方事实]** OpenAPI 的常见响应 envelope 包含 `code/request_id/message/message_zh/time/time_stamp/time_zone/docs/cache_message/cache_url/router/params/data`；具体 `data` 由 endpoint schema 决定。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[官方事实]** 一部分抖音创作者、DOU+ 等接口明确需要用户 Cookie。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[推断]** 需要平台 Cookie 的 endpoint 默认不发布到共享能力目录；只有客户对该账号有合法授权、凭据隔离到企业租户且合同允许时，才单独开放。

### 5.5 错误、重试和不确定结果

- **[官方事实]** 官方 SDK 把 400/422、401、403、404、429、5xx 和 502/503/504 映射为不同错误类型，并保留上游 request ID；认证 header 在错误信息中脱敏。[SDK errors](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/errors.md)
- **[官方事实]** 官方 SDK 默认最多重试 3 次：网络错误、超时、代理错误、429、5xx、502/503/504 会重试；400/401/403/404/422 不重试。[SDK retries](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/retries.md)
- **[官方事实]** 退避约为 0.5、1、2 秒并带 jitter，429 时优先遵循 `Retry-After` 或 `X-RateLimit-Reset`。[SDK retries](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/retries.md)
- **[官方事实]** 官方 Pricing 只保证非 200 不计费；并没有声明“所有 HTTP 200 都一定含有效平台数据”。[Pricing](https://tikhub.io/pricing)
- **[推断]** vSearch 必须同时校验 HTTP status、TikHub envelope 和 endpoint `data` 内的平台状态；不能把 HTTP 200 直接归类为业务成功。
- **[推断]** 对已发出但超时的请求，自动跨供应商 fallback 可能产生两次收费和两个不同数据快照。没有上游幂等键的官方保证时，应标记 `result_unknown` 并对账。

### 5.6 分页、异步和 Webhook

- **[官方事实]** 官方 SDK 文档列出 cursor、page 和 offset 三种分页风格，具体由 endpoint 决定。[SDK pagination](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/pagination.md)
- **[官方事实]** 当前官方 OpenAPI 没有顶层 webhook 定义。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[推断]** 标准 API 的 `page.cursor` 只能由适配器包装上游真实 cursor，不能在不同供应商之间互换。
- **[未知]** 异步字幕、下载或任务型 endpoint 是否向 Enterprise 提供 callback/webhook，需要按 endpoint 向供应商确认；当前产品应以轮询作为基线。

### 5.7 数据授权、隐私与转售边界

- **[官方事实]** 当前用户后台条款授予有限、不可转让、不可再许可、可撤销的内部业务使用许可；未经书面授权不得销售、转售、出租、再分发服务，也不得用服务构建竞争产品。[Current Terms](https://user.tikhub.io/terms)
- **[官方事实]** TikHub 官网同时说明其提供数据访问基础设施，不拥有、转售或再分发底层数据，最终用户负责后续使用、保存和处理。[Pricing footer](https://tikhub.io/pricing)
- **[官方事实]** 当前隐私政策称 API usage logs 包含访问 endpoint、时间、响应码和用量；API 返回的公开平台数据可能被缓存并用于聚合/去标识化数据，客户可按政策说明选择退出相关第三方数据共享。[Privacy](https://user.tikhub.io/privacy)
- **[官方事实]** 隐私政策称分享聚合数据前会剔除 request ID、API key、时间戳、IP 等客户识别元数据；同时客户需自行承担中国平台数据的下游 PIPL 等合规责任。[Privacy](https://user.tikhub.io/privacy)
- **[官方事实]** 官方同时保留一份 Apifox 条款页，其更新时间和表述与当前用户后台条款并不完全一致。[Apifox Terms](https://docs.tikhub.io/5508540m0)、[Current Terms](https://user.tikhub.io/terms)
- **[未知]** 两份条款对既有账号的优先级、可否通过 Enterprise 合同覆盖、原始响应保留期限、DPA、跨境路径和供应商子处理商，需要书面澄清。
- **[推断]** 风险控制应以更严格的当前用户后台条款为最低基线：拿到明确 reseller/white-label addendum 前，不对外售卖 TikHub 衍生能力。

## 6. 重叠与互补

### 6.1 已确认重叠的平台族

**[官方事实]** 两家公开目录都包含以下平台族：[JustOneAPI catalog](https://docs.justoneapi.com/en/)、[TikHub OpenAPI](https://api.tikhub.io/openapi.json)

- 抖音 / Douyin
- TikTok 与部分 TikTok Shop
- 小红书 / RedNote 与蒲公英
- 快手
- 微信公众号 / 视频号相关能力
- 微博
- Bilibili
- Instagram
- YouTube
- Twitter / X
- Reddit
- LinkedIn
- 今日头条
- 知乎

**[推断]** “平台重叠”不代表“operation 等价”。例如同为“帖子详情”，可能分别接受分享 URL、平台 ID 或短链，响应字段、评论计数口径、刷新时间和分页也可能不同。

### 6.2 TikHub 更强的方向

- **[官方事实]** TikHub 对社交平台拆分出大量 Web/App/搜索/榜单/创作者/电商/广告/指数等 API 族。[OpenAPI](https://api.tikhub.io/openapi.json)
- **[官方事实]** 公开了逐请求美元价格范围、日用量折扣、每路径 RPS 规则和付费 RPS 档位。[Pricing](https://tikhub.io/pricing)
- **[官方事实]** 提供 Bearer auth、官方 SDK 的错误类型、重试、分页和 request ID 约定。[SDK docs](https://github.com/TikHub/TikHub-API-Python-SDK/tree/main/docs)
- **[推断]** 在取得转售授权后，重社交平台应优先选择 TikHub 作为主路由，再对少量等价 operation 做 JustOneAPI 备路。

### 6.3 JustOneAPI 更强的方向

- **[官方事实]** JustOneAPI 的公开目录包含 TikHub 核心定位之外的大量电商和非社交垂直能力，例如淘宝天猫、京东、1688、Temu、Shopee、Amazon、闲鱼、得物、贝壳、豆瓣电影、IMDb、YOUKU 和跨平台搜索。[Official catalog](https://docs.justoneapi.com/en/)
- **[推断]** 这些能力应直接绑定 JustOneAPI 的真实 path/schema，不应为了“统一”而伪装成 TikHub operation 或不存在的标准接口。

### 6.4 禁止自动互备的情况

以下任一条件不满足，就不能把两个 endpoint 配成主备：

1. **[推断]** 规范化请求字段和默认值相同。
2. **[推断]** 响应实体、字段含义、时间单位和缺失语义相同。
3. **[推断]** 分页 cursor 可独立开始，且 fallback 不会续接另一家的 cursor。
4. **[推断]** 新鲜度、地域、语言和内容范围相同。
5. **[推断]** HTTP/业务成功、空结果和错误的判定相同。
6. **[推断]** 重试不会造成无法接受的双重计费。
7. **[推断]** 两家合同都允许该数据向当前企业客户交付。

## 7. 面向 gate-relay / vSearch 的标准服务设计

### 7.1 不把供应商 API 名直接暴露为标准能力

**[推断]** 对外 operation 使用稳定业务语义，例如：

- `social.content.get`
- `social.content.search`
- `social.comment.list`
- `social.account.get`
- `social.account.contents.list`
- `social.trend.list`
- `commerce.product.get`
- `commerce.product.search`
- `content.article.search`

每个标准 operation 可有多个 `ProviderOperationBinding`，但 binding 必须保存真实供应商、method、path、operationId、schema hash 和版本；无法无损规范化的字段放在显式 `provider_extensions`，不能悄悄丢弃。

### 7.2 推荐的上游对象

```text
ProviderAccount
  provider                 justoneapi | tikhub
  secret_ref               encrypted reference only
  base_url                 region-aware, allowlisted
  plan                     payg / rps tier / enterprise
  credential_scope
  billing_currency
  status

ProviderOperationBinding
  canonical_operation
  provider_account_id
  http_method
  upstream_path
  upstream_operation_id
  request_schema_hash
  response_schema_hash
  upstream_version
  unit_price_minor
  price_currency
  price_effective_at
  billing_trigger
  rate_limit_rps
  concurrency_limit
  timeout_policy
  retry_policy
  pagination_mode
  legal_scope
  publish_status
```

**[推断]** 目录连接器接口应围绕稳定概念，而不是把所有供应商都塞进 MCP 工具名：

```text
Probe(account)
SnapshotCatalog(account) -> provider catalog snapshot
Describe(binding) -> exact request/response contract
Execute(binding, request) -> normalized result + raw audit metadata
Reconcile(provider_request_id) -> billing/result status when supported
```

### 7.3 标准响应

```json
{
  "request_id": "vr_...",
  "operation": "social.content.search",
  "data": {},
  "page": {
    "cursor": null,
    "has_more": false
  },
  "meta": {
    "source_platform": "douyin",
    "freshness": "realtime"
  }
}
```

**[推断]** 用户响应默认不暴露上游 token、原始 URL、内部错误栈和成本；管理员审计记录另存 `provider`、`provider_request_id`、真实 business code、schema hash、计费结果和脱敏原始摘要。

### 7.4 多币种成本和客户定价

- **[官方事实]** TikHub 公开价格以美元标价。[Pricing](https://tikhub.io/pricing)
- **[未知]** JustOneAPI 公共资料没有明确统一结算币种，必须以登录后台/账单的实际币种为准。[Usage](https://docs.justoneapi.com/en/usage)
- **[推断]** `upstream_cost_micros` 不能在无币种时跨供应商相加；必须同时保存：
  - 原始金额与 `upstream_currency`；
  - 结算到客户计价币种的金额；
  - `fx_rate`、`fx_source`、`fx_timestamp`；
  - 供应商价格快照与折扣档位。
- **[推断]** 客户预扣、上游执行、最终结算和退款/差额必须是同一幂等状态机；结果未知时不应立即确认收入或自动退回后再二次扣款。

### 7.5 路由、限流与熔断

- **[推断]** 路由单位是 operation binding，不是供应商账号整体。
- **[推断]** 健康度、成功率、P95、空结果率、429、平台业务失败和余额不足分别统计；不能用 HTTP 200 成功率掩盖业务失败。
- **[推断]** TikHub 按 path 限流；JustOneAPI 按 endpoint 可能有独立额度，因此限流键使用 `tenant + provider + account + path`。
- **[推断]** fallback 前必须确认请求尚未被上游执行；不确定时返回 `result_unknown`，不自动跨供应商重放。
- **[推断]** 只有标记为 `contract_equivalent=true` 且法律范围一致的 binding 才能参与自动 fallback。

### 7.6 目录同步

1. **[推断]** 定期抓取官方 OpenAPI，保存 raw snapshot、ETag/Last-Modified（若有）、hash 和版本。
2. **[推断]** 生成新增、删除、deprecated、method/path、request schema、response schema 和 security diff。
3. **[推断]** 新增/变化接口默认 `draft`，管理员确认价格、权限、参数和测试后才 `published`。
4. **[推断]** 同步目录不等于启用全部接口；需要平台 Cookie、违反产品政策、价格未知或 schema 不完整的接口默认禁用。
5. **[推断]** 线上执行只使用已审核 schema hash，防止供应商原地改 schema 后静默破坏客户契约。

### 7.7 安全与审计

- **[推断]** secret 只保存在服务端加密存储，不进入前端、导出 CSV、业务日志或错误详情。
- **[推断]** JustOneAPI 的 query token 在入口、出口、代理、APM 和审计层统一 redaction。
- **[推断]** TikHub Bearer header 必须在日志、错误对象和 tracing 中脱敏；官方 SDK 已遵循 auth header redaction，可作为实现参考。[SDK errors](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/errors.md)
- **[推断]** 记录 `provider_request_id`、本地 request ID、租户、binding、schema hash、参数 hash、上游状态、是否计费和结算状态；原始个人数据按最小必要原则保存。

## 8. 采购建议

### 8.1 现在怎么买

| 阶段 | JustOneAPI | TikHub | 是否对外售卖 |
| --- | --- | --- | --- |
| 文档/目录验证 | 注册免费试用 | 使用 `$0.05` 免费额度 | 否 |
| 小规模真实调用 | 只做小额充值，先确认逐接口价格与币种 | 使用 PayG；默认 10 RPS 足够验证 | 否 |
| 容量验证 | 根据真实高峰确认 endpoint 限额 | 单一路径持续逼近 10 RPS 后，再从 20 RPS `$5/month` 起升档 | 否 |
| 商业开放 | 取得 reseller/white-label/SaaS 书面授权、价格表、SLA、DPA | 取得覆盖当前 Terms 限制的书面 addendum | 是，但仅限合同明确允许的能力 |
| Enterprise | 有明确折扣/SLA/授权收益后再购买 | 接近实际消费门槛或需要私有部署/专属支持时再评估 | 仍取决于书面授权 |

**[推断]** 不建议现在一次性向 TikHub 充值 `$3,000`，也不建议在 JustOneAPI 价格、币种和商用授权未知时大额充值。先用真实业务样本测出成功率、空结果率、P95、单次成本和平台覆盖，再决定套餐。

### 8.2 买会员前必须让两家书面回答

1. 是否允许把 API 结果经过字段标准化后，通过我们自己的付费 API/SaaS 返回给企业客户？
2. 是否允许 white-label、API resale、sub-license、multi-tenant downstream delivery？允许范围是原始结果、派生字段、聚合统计还是仅内部展示？
3. 每个平台分别有哪些禁止用途、保留期限、地域/跨境限制、版权或个人信息限制？
4. 是否提供 DPA、子处理商清单、数据存储地区、删除 SLA 和安全事件通知？
5. endpoint 价格、币种、折扣和计费触发条件能否通过机器接口导出？价格变更提前多久通知？
6. timeout、连接中断、重复请求和平台返回空数据时如何计费/退款？能否按 request ID 对账？
7. RPS 是按账号、token、path 还是 IP？burst、并发、429 header 和升档生效时间是什么？
8. 能否为 production/staging/catalog sync 分配独立 key、scope、预算和 IP allowlist？
9. 是否有 uptime SLA、状态页、维护通知、技术支持响应时限和重大 endpoint 变更通知？
10. 是否有通用 webhook、异步任务 callback 或账单事件 webhook？
11. 是否允许我们在目录中展示供应商/平台名称、logo 和数据来源说明？是否有强制 attribution？
12. 合同终止后，已返回数据、缓存、派生统计和客户审计记录如何处理？

## 9. 实施阶段与验收门槛

### Phase 0：商业与合规门槛

- 两家都拿到书面 downstream/reseller 权利。
- 明确平台级数据使用限制、DPA、跨境和保留期限。
- 明确价格、币种、计费触发、退款与 request ID 对账。
- **未完成时只允许内部试点。**

### Phase 1：目录与账号

- 建立 `justoneapi_rest`、`tikhub_rest` 两种真实 provider adapter。
- 导入官方 OpenAPI 快照并生成 diff，不自动发布。
- 保存账号 base URL、区域、plan、币种、RPS 与凭据引用。
- 管理端能查看 catalog version/hash、价格更新时间和合法发布范围。

### Phase 2：最小可售能力

- 每家先选 5–10 个高价值、schema 稳定、商用允许的 endpoint。
- 为每个 binding 完成请求、响应、分页、空结果、错误、429、超时和计费契约测试。
- 真实调用对账：本地 usage、供应商 request ID、上游账单与客户扣费一致。
- 任何一次不确定结果都进入 reconciliation，而不是自动重复扣费。

### Phase 3：等价路由与容量

- 只有契约测试证明等价的 operation 才配置双上游。
- 压测按 endpoint/path 进行，验证本地限流低于供应商额度。
- 故障演练覆盖 429、5xx、业务失败、空数据、账号余额不足、schema 变化和区域域名不可达。
- RPS 套餐仅根据真实观测升级。

### Phase 4：企业开放

- 客户协议反映第三方平台数据限制和禁止用途。
- 管理端按企业授权 capability，默认最小权限。
- 对外页面明确数据来源类别、时效、计费单位、错误语义和保留政策。
- 建立定期价格、条款、OpenAPI、账单和数据合规复核。

## 10. 仍未确认的关键风险

1. **[未知] 转售权**：JustOneAPI 没有公开授权；TikHub 当前条款默认限制转售。
2. **[未知] 条款冲突**：TikHub 两份官方条款页面的适用优先级需书面确认。
3. **[未知] JustOneAPI 商业参数**：公开资料没有币种、会员、折扣、SLA、并发和退款规则。
4. **[未知] 数据保留/DPA**：两家的 API payload 保存、处理地、子处理商和删除机制仍不完整。
5. **[未知] Webhook**：两家 OpenAPI 都没有通用 webhook；异步 endpoint 的 callback 需逐一确认。
6. **[推断] 双重计费**：没有供应商幂等键保证时，timeout 后跨供应商重放可能造成双重成本。
7. **[推断] Schema 漂移**：TikHub 实时 OpenAPI 和 SDK 生成目录已有数量差异；目录必须版本化审查。
8. **[推断] 平台条款风险**：供应商可提供接口不代表目标平台允许我们的特定下游用途。

## 11. 官方来源清单

### JustOneAPI

- [Official website](https://justoneapi.com/en)
- [API Usage Guide](https://docs.justoneapi.com/en/usage)
- [Official API catalog](https://docs.justoneapi.com/en/)
- [Official OpenAPI](https://docs.justoneapi.com/openapi.json)
- [FAQ](https://justoneapi.com/en/faq)
- [Terms of Service](https://justoneapi.com/en/terms)
- [Privacy Policy](https://justoneapi.com/en/privacy)
- [Official Python SDK](https://github.com/justoneapi/justoneapi-python)
- [Official SDK transport](https://github.com/justoneapi/justoneapi-python/blob/main/justoneapi/_transport.py)
- [Official MCP server](https://github.com/justoneapi/justoneapi-mcp)
- [Official MCP tools](https://github.com/justoneapi/justoneapi-mcp/blob/main/TOOLS.md)

### TikHub

- [Official website](https://tikhub.io/)
- [Pricing and RPS plans](https://tikhub.io/pricing)
- [Getting Started](https://tikhub.io/getting-started)
- [Swagger UI](https://api.tikhub.io/)
- [Official OpenAPI](https://api.tikhub.io/openapi.json)
- [Current Terms of Service](https://user.tikhub.io/terms)
- [Current Privacy Policy](https://user.tikhub.io/privacy)
- [Alternate official Apifox Terms page](https://docs.tikhub.io/5508540m0)
- [Official Python SDK](https://github.com/TikHub/TikHub-API-Python-SDK)
- [SDK authentication](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/authentication.md)
- [SDK errors](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/errors.md)
- [SDK retries and rate limits](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/retries.md)
- [SDK pagination](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/pagination.md)
- [SDK generated reference](https://github.com/TikHub/TikHub-API-Python-SDK/blob/main/docs/reference.md)
