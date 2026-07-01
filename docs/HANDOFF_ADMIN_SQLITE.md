# TransBridge 管理后台与 SQLite 持久化交接文档

## 目标

本次改造为 TransBridge 增加本地 Web 管理后台和 SQLite 持久化能力，并补强翻译质量防护：

- 页面添加模型后持久保存，并热重载运行时模型。
- 管理 translate / openai / all scope 的访问 token。
- 保存 prompt 历史版本，并切换 active prompt。
- 保存请求统计和历史日志。
- 服务重启后保留后台配置。
- 模型输出明显不可用时不写入 bbolt/Redis/memory 缓存。
- 对普通文本也应用保守翻译策略，跳过不应翻译的数据、单位、化学式、缩写等内容。

## 主要文件

| 文件 | 说明 |
|------|------|
| `config/config.go` | 新增 `storage` 和 `admin` 配置结构 |
| `store/store.go` | SQLite 打开、迁移、CRUD、统计、日志 |
| `admin/handler.go` | `/admin/` 和 `/admin/api/*` 后台接口 |
| `admin/ui.go` | 内置 HTML 管理页面 |
| `service/policy.go` | 翻译前策略：skip / dictionary / model |
| `service/quality.go` | 模型输出质量门禁，决定是否允许写缓存 |
| `service/translation.go` | 接入策略、模型级缓存 key、SQLite 请求日志 |
| `translator/model_manager.go` | 新增 `Reload`，后台保存模型后热重载 |
| `api/deeplx/translate_handler/*` | 支持动态 token 和 active prompt |
| `api/openai/openai_handler.go` | 支持动态 token |
| `internal/utils/utils.go` | 新增模型级缓存 key |

## 配置

示例：

```yaml
storage:
  enabled: true
  type: "sqlite"
  path: "data/transbridge.db"

admin:
  enabled: true
  path: "/admin"
  username: "admin"
  password: "change-me"
  local_only: true
```

首次启动时，如果 SQLite 是空库，会从 YAML 导入：

- `providers`
- `transapi.tokens`
- `openai.compatible_api.auth_tokens`
- `prompt.template`

后续后台修改写入 SQLite。启动时如果 SQLite 已有 provider 或 active prompt，会优先使用 SQLite 数据。

新增 SQLite 驱动 `modernc.org/sqlite v1.36.3`，源码编译最低 Go 版本为 1.25。

## 后台入口

```text
http://127.0.0.1:8080/admin/
```

默认使用 Basic Auth。`local_only: true` 时只允许 loopback 地址访问。

## API 概览

| 路径 | 方法 | 说明 |
|------|------|------|
| `/admin/api/stats` | GET | 请求统计 |
| `/admin/api/models` | GET/POST/DELETE | 模型管理，写入后热重载 |
| `/admin/api/tokens` | GET/POST/PUT/DELETE | token 管理 |
| `/admin/api/prompts` | GET/POST | prompt 版本管理 |
| `/admin/api/prompts/activate?id=1` | POST | 切换 active prompt |
| `/admin/api/logs?limit=100` | GET | 历史请求日志 |

## SQLite 表职责

| 表 | 职责 |
|----|------|
| `providers` | 上游 provider 配置 |
| `models` | 模型参数、权重、启用状态、限流 |
| `tokens` | 访问 token、scope、调用次数、最近使用时间 |
| `prompt_versions` | prompt 历史版本和 active 标记 |
| `request_logs` | 请求统计、缓存命中、模型、耗时、错误 |

SQLite 不保存翻译结果缓存。翻译结果仍由 `memory`、`bbolt`、`redis` 缓存层负责。

## 翻译策略

调用模型前会先分类：

- `skip`：数字、百分比、引用编号、URL、email、单位、复合单位、化学式、离子式、物种名、基因/蛋白符号、日期金额、单字母、短 acronym 等直接保留。
- `dictionary`：中文目标语言下的高置信短词直接翻译，例如 `total -> 总计`、`control -> 对照`。
- `model`：普通短语和长文本才调用模型。

模型返回后，写缓存前会校验：

- 空输出不缓存。
- “请提供文本”“you haven't provided the actual text”等输出不缓存。
- “您提供的文本是”“翻译成简体中文为”“the translation is”等解释性输出不缓存。
- 大量短行断裂且含助手话术的输出不缓存。

新缓存 key 使用：

```text
provider + model + source_lang + target_lang + source_text
```

旧缓存 key 仍兼容读取，但命中后也会经过质量门禁。

## 验证

已执行：

```bash
go test ./...
```

新增测试覆盖：

- 科学/数据 token 跳过。
- 常见短词词典翻译。
- 未知单词保守跳过。
- 解释性、碎片化模型输出不写缓存。
- SQLite 从 YAML 导入 provider、token、prompt。

建议人工验收：

1. 使用启用 `storage` 和 `admin` 的配置启动服务。
2. 访问 `/admin/`，确认 Basic Auth 和 local only 生效。
3. 在后台新增一个模型，刷新 `/v1/models` 确认模型列表变化。
4. 新增一个 translate token，用它调用 `/translate`。
5. 新增并启用一个 prompt，调用翻译确认使用新 prompt。
6. 查看 `/admin/api/logs` 和页面历史日志是否出现请求记录。

## 注意事项

- 管理后台当前是单机本地管理设计，不建议直接暴露公网。
- `admin.local_only` 只检查 HTTP 连接来源地址；如果放在反向代理后面，需要在代理层限制访问。
- token 在 SQLite 中明文保存，生产环境应保护 `data/transbridge.db` 文件权限。
- 后台模型热重载会关闭旧 translator。当前 OpenAI translator 没有长连接状态，风险较低。
- 单个英文短词默认不交给模型，可能导致部分短表头不翻译；可通过后续词典扩展解决。

## 后续建议

1. 增加 token 脱敏显示和复制按钮。
2. 增加模型连通性测试按钮。
3. 增加 prompt 测试面板。
4. 增加请求日志筛选条件：模型、成功失败、缓存命中、时间范围。
5. 增加外部术语词典和策略资源文件。
6. 增加 SQLite schema version 表和显式迁移版本号。
7. 为管理 API 增加更细的单元测试和 httptest 覆盖。
