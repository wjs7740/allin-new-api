# Zhiqi 接入与验收

## 运行本分支

```bash
git checkout feat/zhiqi-platform
docker compose -f compose.allin.yml up -d --build
```

默认访问 `http://127.0.0.1:18463`，首次进入初始化。此配置从本地源码构建，使用独立 SQLite 数据卷；设置 `ALLIN_PORT` 可调整端口。需要公开域名时再配置 HTTPS 反向代理、可信代理、稳定 Session Secret 和 Secure Cookie。现有 `docker-compose.yml` 的镜像配置不会自动使用本分支源码。

生成队列由内置任务执行器处理，须保持 `UPDATE_TASK=true`。数据库迁移会新增素材、提交队列、回执和幂等表。

## 配置顺序

1. 构建并启动本分支，完成 New API 初始化。在“插件”页面启用插件体系，确认内置 `Zhiqi` 已加载。
2. “渠道”中新建 `Plugin / 任务插件（61）` 渠道，绑定 `zhiqi`。Base URL 填 `https://zhiqiapi.com`，密钥填上游 Key。素材使用一个商户 Key 对应一个渠道，避免不同账户的资源混用。
3. 文本和同步图片可从渠道获取模型。视频/异步图片先到“媒体平台 → 上游接入”输入渠道 ID，点击目录预览，确认该 Key 的真实模型名。
4. 将视频和异步图片的模型分别填写进对应文本框，再预览并安装目录。这里只声明接口能力，不自动覆盖已有零售价。接口能力需根据上游文档及实际联调确认。
5. 回到渠道启用所选模型，设置可用用户组；在模型价格中为同步调用设置普通价格，为 `zhiqi` 任务设置独立插件价格。
6. 如需素材库，在接入页选择该渠道作为素材渠道。用户进入素材库新建私有组，管理员可选择公共范围；所有上游组仍由商户账户私有持有，本地系统决定用户可见范围。
7. 创建本站 API Key，读取 `GET /v1/models`，再使用工作台或下面的接口联调。工作台使用本站 Key，真实提交会产生生成费用。

通用别名 `zhiqi-video`、`zhiqi-image` 只用作接入起点，可以通过模型映射关联到真实上游型号。初始目录不声明 GPT 图片等通用型号，避免影响原有渠道的模型定价行为。上游目录安装后如已有同名模型，先检查该模型的普通价格与任务插件价格。

## 接口

| 功能 | 接口 |
| --- | --- |
| 标准视频 | `POST /v1/videos`、`GET /v1/videos/{id}`、`GET /v1/videos/{id}/content` |
| Zhiqi 格式视频 | `POST /zhiqi/v1/videos`、`GET /zhiqi/v1/videos/{id}`、`GET /zhiqi/v1/videos/{id}/content` |
| 异步图片 | `POST /v1/images/generations/async`、`GET /v1/images/generations/{id}` |
| 零售价格目录 | `GET /api/pricing_new`，使用本站 API Key，只返回当前权限可访问模型 |
| 素材分组 | `/api/aigc-assets/groups`、`/api/aigc-assets/groups/{id}` |
| 素材 | `/api/aigc-assets/assets`、`/api/aigc-assets/assets/{id}`、`/preview` |
| 本地文件上传 | `POST /api/aigc-assets/uploads`、`POST /api/aigc-assets/uploads/{id}/complete` |

标准视频成功状态为 `completed`，Zhiqi 格式视频与异步图片为 `succeeded`。创建与查询返回本站任务 ID；查询、内容与素材操作均先验证本站用户身份。原生兼容视频内容接口跳转到有时限的本地媒体代理地址。

```bash
curl https://YOUR_DOMAIN/v1/videos \
  -H 'Authorization: Bearer LOCAL_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_VIDEO_MODEL","prompt":"A fox in a forest","duration":6,"generate_audio":false,"seed":0}'

curl https://YOUR_DOMAIN/v1/images/generations/async \
  -H 'Authorization: Bearer LOCAL_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR_IMAGE_MODEL","prompt":"A blue ceramic cup","n":1,"resolution":"2k"}'
```

素材引用示例：

```json
{
  "model": "YOUR_VIDEO_MODEL",
  "prompt": "The character waves",
  "duration": 6,
  "library_assets": {"images": ["aia_LOCAL_ASSET_UUID"]}
}
```

素材必须就绪、属于同一渠道，并且是当前用户私有素材或公共素材。提交上游之前，本地解析并替换素材 ID。URL 参考图、参考视频和音频须为上游可读取的 HTTP(S) URL。

## 价格

Zhiqi 任务使用插件价格覆盖项，例如 `zhiqi::YOUR_VIDEO_MODEL`。以下数字只是演示本站零售价的表达式写法，不代表上游成本：

```text
按请求：tier("request", u("requests") * 0.10)
按视频秒：tier("video", u("seconds") * 0.02)
按图片张：tier("image", u("image_count") * 0.03)
分辨率档：u("resolution") == "2k" ? tier("2k", u("image_count") * 0.06) : tier("default", u("image_count") * 0.03)
```

每个任务提交时冻结价格。视频未传时长使用渠道默认值（初始 6 秒），提交和计价使用同一个值；图片按有效 URL/Base64 载荷结算，纯提示词条目不计图。普通任务提供请求数、秒数、图片数、分辨率四个事实。Seedance fast-token / mini-token 使用独立用量配置：按上游目录每秒 20,000 Token 预估，完成后按有效 usage.total_tokens 结算；参考视频输入可以单独定价，缺少有效用量保留预扣。

同步图片和文本继续使用原有模型计费机制。上游 `/api/pricing_new` 只作为管理员核对成本的来源，零售目录导出的是本站价格。

## 异常与核对

- 本地在向上游发送媒体生成请求之前保存已预扣的提交记录。
- 网络失败、408、429、5xx、成功响应缺 ID 时，保留本地任务等待核对；该次请求不自动重新生成或跨渠道提交。
- 查询失败和任务超时不会自动把 Zhiqi 已消费的生成费用退回。任务记录继续保留，管理员核对上游状态。
- 上游明确返回生成失败时使用已有失败退款流程；`media_storage_failed`、`media_storage_unavailable` 保留生成消费，交付错误不会触发再次生成。
- 管理员在接入页输入已核实的本地任务 ID 和上游任务 ID，可关联回执并恢复轮询。接口 `POST /api/zhiqi/tasks/reconcile` 仅接受 root 浏览器会话，检查渠道、模型以及回执唯一归属，不创建新生成任务。
- 素材变更使用持久化 `request_id` 和 `lock_version`。结果不确定时保留本地资源 ID，不盲目重建；先刷新或核对上游。

## 当前实现范围与实网验收

已经实现：提供商转发、标准/原生视频、异步图片、目录预览与插件配置、本站价格目录、素材所有权映射与 CRUD/上传/预览/视频引用、媒体工作台、任务核对入口。

当前使用持久化提交队列：校验与预扣完成后保存任务和请求，返回本地任务 ID，由后台轮询执行器领取队列并提交上游。领取使用条件更新，进程重启后继续处理未领取的任务；已领取而结果不明的任务进入待核对状态，保持费用且不自动重复提交。请求载荷在提交结束后清理。视频和异步图片支持 `Idempotency-Key`：同一用户、API Key、接口、幂等键与 JSON 请求返回同一任务；相同键对应不同请求返回 409。首个请求尚未保存任务时返回 409 和 Retry-After，客户端保留原幂等键重试。工作台在失败重试时保留幂等键。

媒体稳定 URL、对象存储和官方素材处理由 Zhiqi 提供；本地实现鉴权代理与权限映射。自建 S3 媒体转存、完整可编辑文档 CMS 是设计中的后续独立扩展，不作为本版本已经交付的能力。

上线前需要配置真实 Key，验收每个模型的可见性、参考素材、生成/查询/下载及费用对账；公开文档与本地模拟测试不替代这些检查。图片 SSE、多规格和上传限制以具体上游模型实测为准。

## 本地验证

- 后端相关包：`go test ./controller ./router ./relay ./service ./plugins ./model ./relay/channel/task/jsplugin -count=1`
- 数据库矩阵：设置独立的 `ALLIN_TEST_MYSQL_DSN`、`ALLIN_TEST_POSTGRES_DSN` 后运行 `go test ./model -run TestZhiqiAssetDatabaseContracts -count=1 -v`。
- 前端：`bun check -b`、受影响文件的 `oxlint`、`bun run test src/features/zhiqi/__tests__/workbench.test.tsx`、`bun run build`。
- 浏览器：登录、媒体页面、接入页、素材空状态与移动视口；上游 Key 不进入浏览器持久化存储。

数据库验证使用 SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 17.11，覆盖新表、重复迁移、已有数据保留、请求幂等、用户隔离、乐观锁、回执唯一绑定。没有在业务数据库上执行测试。

权限设计参考 OWASP ASVS 5.0.0 的访问控制原则，以及 Authentication / Session Management Cheat Sheets；复用现有登录会话中间件，新增管理 API 仅接受会话，API 素材访问使用本站令牌并执行资源归属检查。该记录不是对整个继承项目的全面合规认证。
