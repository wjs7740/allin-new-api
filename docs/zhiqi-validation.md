# Zhiqi 二开验收记录

基线：New API `v1.0.0-rc.51` / `c9c6da2786b6a0912215127e78e28fbc6650c85f`。
开发仓库：`wjs7740/allin-new-api`，分支 `feat/zhiqi-platform`。
验证日期：2026-11-13 至 2026-11-14。

## 已执行

| 验证 | 结果 |
| --- | --- |
| `go test ./controller ./router ./relay ./service ./plugins ./model ./relay/channel/task/jsplugin ./relay/channel/providerplugin -count=1` | 8 个包通过 |
| `go test ./model -run '^TestZhiqiAssetDatabaseContracts$' -count=1 -v`，配置 MySQL/PostgreSQL 测试 DSN | SQLite 3.50.4、MySQL 8.0.46、PostgreSQL 17.11 通过 |
| 最新素材操作保护：`go test ./controller -run '^TestZhiqi' -count=1` | 通过 |
| `bun check -b` | 类型检查通过 |
| `bun x oxlint -c .oxlintrc.json src/features/zhiqi src/routes/_authenticated/media src/hooks/use-sidebar-data.ts` | 通过 |
| `bun run test src/features/zhiqi/__tests__` | 2 个文件、4 项行为测试通过 |
| `bun run build` 和 Go 可执行文件构建 | 通过 |
| Playwright 本地浏览器检查 | 登录、媒体工作台、接入页、素材空状态、手机视口均可用，无页面异常 |
| 独立实例 + 本地模拟 zhiqi HTTP | 视频、异步图片均完成创建、后台领取、提交、轮询与终态查询；视频内容下载通过 |
| HTTP 幂等重复提交 | 相同幂等键与请求返回同一任务 ID；两类任务各只向上游提交一次 |

## 关键保护范围

- 图片对象/数组/拆分载荷按有效图片计数；无图片载荷保留预扣事实。
- 视频参数别名冲突、数量与时长上界、布尔和零值保留。
- 同步图片保留 `resolution`、`aspect_ratio` 等扩展字段；multipart 编辑保留多张 URL 参考图。
- 私有素材按本地 owner 验证，公共素材由管理员维护；直接猜测上游素材 ID、跨用户和跨渠道引用均受校验。
- 素材创建请求幂等、乐观锁与变更期间刷新保护；组删除清理子资源并推进版本。
- 提交队列先持久化，账务完成后激活；只有成功领取的执行者提交。失联领取进入待核对，不自动重发。
- 上游明确拒绝走退款；交付失败保留生成费用；查询错误和超时不推断生成失败。
- 任务回执通过独立唯一索引防止重复绑定；管理员核对接口排除仍在提交的任务。
- 本地 API Key 与上游商户 Key 分离；商户 Key 不跟随重定向，不进入公开任务或素材响应。
- 模型目录只生成管理员选定且当前上游可见的型号；成本预览不自动覆盖本站售价。

## 验收边界

所有生成请求均使用本地模拟上游，没有调用付费模型。实际 Zhiqi Key、Muse/MiniMax 型号、价格单位、图片 SSE 和云存储上传需配置后按供应商实际回包验收。

媒体存储、官方素材处理由 Zhiqi API 承担，本地负责任务与素材权限、代理下载和计费。自建 S3 转存、完整文档 CMS 和自动按上游视频 Token 结算不属于当前已实现功能；设计文档保留其扩展方向。

用户界面复用了 `SectionPageLayout`、`ConfirmDialog`、`CopyButton`、`EmptyState`、`LoadingState`、`DataTablePagination` 及已有基础控件。素材分页使用现有 TanStack 表格状态；文件上传使用现有 Input 的 file 类型。没有新增 UI 框架。

测试数据库和预览位于独立容器及 `/tmp/allin-preview-data`。预览仅绑定回环地址；本地模拟 Key 不进入仓库。生产配置参考 `zhiqi-setup.md`，真实 Key 通过渠道后台输入。
