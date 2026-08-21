# 外部 API 契约验证（Contract Gate）

> Phase 3 允许的唯一停顿点。本项目外部依赖仅 SMTP。

| Provider | 用途 | 验证时间 | 状态 |
|---|---|---|---|
| SMTP（Mailpit / 标准 net/smtp） | 任务进入已完成列时通知项目 PM | 2026-08-21 20:31 GMT+8 | **VERIFIED**（`/api/health` smtp=true；compose 内真实 `net/smtp` → Mailpit:1025） |

## SMTP 约定（不猜测专有 JSON）

- 协议：SMTP（RFC 5321），Go 标准库 `net/smtp`
- 开发收件端：`mailpit:1025`，无 TLS，AUTH 可空
- 真实外发：改 `SMTP_HOST/PORT/USER/PASS/TLS`，代码路径不变
- 成功：SMTP 2xx/250
- 瞬时失败：网络超时、4xx 临时拒绝、连接重置 → 最多 3 次指数退避
- 永久失败：5xx 邮箱不存在、认证失败 → **不重试**，死信
- 单价：自托管，¥0

切换方式见未来 README §7。Mock 合法性：真实 SMTP 栈 + 仅收件服务端不同。
