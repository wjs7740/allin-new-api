# new.allincode.top 部署记录

- 站点：https://new.allincode.top
- API Base URL：https://new.allincode.top/v1
- 注册：https://new.allincode.top/sign-up
- 文档：https://new.allincode.top/docs
- DNS A：84.32.220.181；检查时无 AAAA 记录。
- Nginx 独立虚拟主机：`/etc/nginx/sites-enabled/new.allincode.top`，目标为 `/root/allin-new-api-deploy/nginx-domain.conf`。
- 上游：`http://127.0.0.1:18463`。保留 200 MB 请求上限、3600 秒媒体调用超时、WebSocket 转发与关闭代理缓冲。
- 域名 HTTP 跳转 HTTPS；旧公网 `84.32.220.181:18463` 以 308 跳转新域名。API 客户端应直接使用新 HTTPS Base URL。
- Let's Encrypt 证书路径：`/etc/letsencrypt/live/new.allincode.top/`；本次签发有效至 2027-02-13。`certbot.timer` 已启用，续期使用 Nginx authenticator/installer。
- 应用 `ServerAddress=https://new.allincode.top`。
- `RegisterEnabled=true`、`PasswordRegisterEnabled=true`；保留 `EmailVerificationEnabled=false`、`QuotaForNewUser=0`。
- 环境变量 `SESSION_COOKIE_SECURE=true`、`SESSION_COOKIE_TRUSTED_URL=https://new.allincode.top`。沿用现有 HttpOnly、SameSite=Strict、host-only Cookie 和 Origin 检查实现。

本次为部署和开关调整，没有改动认证代码或数据库模型。TLS 和 Cookie 配置参考 [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html#transmit-passwords-only-over-tls-or-other-strong-transport) 与 [Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html#cookies)，不代表对整个继承项目的全面合规认证。

验收文件在部署目录：`https-api-verification.json`、`https-registration-verification.json`。检查包括公开页面、16 个授权模型、本站地址、注册开关及临时用户的注册/登录/刷新/退出；测试账号使用自身密码二次验证后注销。密钥和测试密码不进入仓库。
