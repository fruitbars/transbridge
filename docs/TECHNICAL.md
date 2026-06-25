# TransBridge 技术文档

本文面向维护者、贡献者和需要二次开发的部署者，说明 TransBridge 的设计目标、代码结构、请求链路、缓存模型、上游协议、测试发布流程和扩展建议。

## 设计目标

TransBridge 是一个轻量级翻译 API 代理。核心目标是把多个 OpenAI-compatible 大模型服务统一封装成稳定的翻译接口，并提供面向常见客户端的兼容 API。

项目当前坚持几个约束：

- 上游统一使用 OpenAI-compatible `/v1/chat/completions` 协议。
- 对外提供简单翻译接口、沉浸式翻译批量接口和 OpenAI-compatible 代理接口。
- 单机部署默认使用 `memory + bbolt` 缓存，避免强依赖 Redis。
- 多实例部署仍可使用 Redis 作为共享缓存。
- 代码保持小而直接，优先用标准库和少量稳定依赖。

## 目录结构

```text
.
├── main.go                         # 程序入口、组件初始化、HTTP 路由注册
├── api/
│   ├── deeplx/translate_handler/    # /translate、/deepl/v2/translate 和 /immersivel 接口
│   └── openai/                      # /v1/chat/completions 和 /v1/models
├── service/
│   └── translation.go               # 翻译业务编排：缓存、模型选择、日志
├── translator/
│   ├── translator.go                # Translator 接口
│   ├── openai.go                    # OpenAI-compatible 上游实现
│   └── model_manager.go             # 模型注册、默认模型、权重选择
├── cache/
│   ├── cache.go                     # Cache 接口和缓存记录结构
│   ├── memory.go                    # 内存缓存
│   ├── bbolt.go                     # bbolt 本地持久化缓存
│   ├── redis.go                     # Redis 缓存
│   └── multi.go                     # 多级缓存组合
├── config/
│   ├── config.go                    # YAML 配置结构
│   └── ttl.go                       # TTL 字符串解析
├── internal/
│   ├── middleware/                  # CORS、日志、恢复、限流中间件
│   └── utils/                       # 缓存 key、提示词模板、语言工具
├── logger/
│   └── translation_logger.go        # 异步结构化翻译日志
├── docs/                            # 用户文档和技术文档
├── .github/workflows/               # CI 和 Release 工作流
└── config.example.yml               # 标准示例配置
```

## 运行时架构

```mermaid
flowchart LR
    Client[Client] --> HTTP[HTTP Server / middleware]
    HTTP --> DeepL[/translate / /deepl/v2/translate / /immersivel]
    HTTP --> OAI[/v1/chat/completions / /v1/models]
    DeepL --> Service[TranslationService]
    Service --> Cache[MultiCache]
    Service --> Manager[ModelManager]
    Manager --> Translator[OpenAITranslator]
    Translator --> Upstream[OpenAI-compatible upstream]
    Service --> TLog[TranslationLogger]
    Cache --> Memory[Memory]
    Cache --> Bolt[bbolt]
    Cache --> Redis[Redis]
```

主要请求路径有三条：

- `/translate`：项目简化单条翻译接口。
- `/deepl/v2/translate`：DeepL v2 风格 JSON 接口，`text` 为数组，响应为 `translations` 数组。
- `/immersivel`：沉浸式翻译批量接口，最多 50 条文本，默认并发 5。
- `/v1/chat/completions`：OpenAI-compatible 代理接口，透传聊天完成请求。

## 初始化流程

`main.go` 的启动流程：

1. 读取 `-config` 参数，默认 `config.yml`。
2. 使用 `config.LoadConfig` 加载 YAML。
3. 根据 `cache.enabled` 和 `cache.types` 初始化缓存。
4. 根据 `log.enabled` 初始化异步翻译日志。
5. 根据 `providers` 初始化 `ModelManager`。
6. 创建 `TranslationService`。
7. 注册 HTTP 路由。
8. 启动 HTTP server。
9. 收到 `SIGINT` 或 `SIGTERM` 后优雅关闭 server、logger 和 cache。

HTTP server 当前固定超时：

```go
ReadTimeout:  15 * time.Second
WriteTimeout: 15 * time.Second
IdleTimeout:  60 * time.Second
```

## 配置模型

标准配置入口是 `config.example.yml`。历史配置文件 `config_transbrige.yml` 和 `config_transbrige_ollama.yml` 保留为示例兼容。

### Provider

当前所有上游都通过 OpenAI-compatible 协议接入，所以 `provider` 固定为 `openai`：

```yaml
providers:
  - provider: "openai"
    api_url: "http://localhost:11434/v1/chat/completions"
    api_key: "ollama"
    timeout: 60
    is_default: true
    models:
      - name: "llama3.1:latest"
        weight: 10
        max_tokens: 2000
        temperature: 0.3
```

`ModelManager` 使用 `provider + model + api_url` 作为内部模型标识。对外 OpenAI-compatible 模型列表返回 `provider/model`，例如：

```text
openai/gpt-4o-mini
openai/llama3.1:latest
```

### Cache

缓存配置支持：

- `memory`：进程内缓存，读写最快，重启丢失。
- `bbolt`：本地文件持久化缓存，适合单机部署。
- `redis`：外部共享缓存，适合多实例部署。

推荐组合：

```yaml
cache:
  enabled: true
  types: ["memory", "bbolt"]
```

多实例部署：

```yaml
cache:
  enabled: true
  types: ["memory", "redis"]
```

`MultiCache` 会按配置顺序读取缓存。命中后，会把较低层命中的值回填到前面的缓存层。

## 翻译请求链路

`/translate` 请求链路：

```text
HTTP handler
  -> validate method/auth/body
  -> TranslationService.Translate(ctx, ...)
      -> generate cache key
      -> try cache
      -> select translator by weight
      -> apply prompt template
      -> call upstream OpenAI-compatible API
      -> write cache
      -> enqueue translation log
  -> JSON response
```

关键点：

- 缓存 key 基于 `source_lang + target_lang + text` 生成。
- `prompt.template` 必须包含 `{{input}}`。
- `source_lang` 和 `target_lang` 会转换成语言名称后注入提示词。
- `TranslationService` 会把 request context 传到 translator，上游 HTTP 请求支持取消和超时。
- 如果缓存中存在旧拼写前缀 `transbrige:` 的 key，会尝试兼容读取。

## OpenAI-compatible 代理链路

`/v1/chat/completions` 不走翻译提示词模板，也不走翻译缓存。它是一个上游聊天完成代理：

```text
HTTP handler
  -> auth
  -> parse ChatCompletionRequest
  -> parse model as provider/model
  -> find configured OpenAITranslator
  -> replace request model with upstream model name
  -> forward request to upstream
  -> return upstream response
```

如果模型不存在，会返回 `400 invalid_model`，不会 fallback 到默认模型。这是为了保持 OpenAI-compatible 调用方的可预期性。

## 缓存实现细节

### MemoryCache

`MemoryCache` 使用 `map[string]memoryItem` 和 `sync.RWMutex`。过期项由两个机制清理：

- `Get` 时发现过期项会删除。
- 后台 goroutine 每分钟扫描一次过期项。

容量达到 `max_size` 时，会删除 map 中第一个遍历到的元素。当前不是 LRU，这一点实现简单但不适合需要精确淘汰策略的场景。

### BboltCache

`BboltCache` 使用一个 bucket：

```text
translations
```

每个值存储为：

```json
{
  "data": "serialized CacheEntry",
  "expire_at": 1730000000000000000
}
```

`expire_at` 为 Unix nano；为 0 表示永不过期。读取时发现过期会删除该 key。

### RedisCache

`RedisCache` 直接使用 Redis key/value 和 Redis 自身 TTL。`ttl == 0` 时使用配置默认 TTL；`ttl < 0` 或 permanent 时设置为不过期。

## 模型选择策略

`ModelManager` 在启动时按配置顺序注册模型。

- 显式 `is_default: true` 的 provider，其第一个模型是默认模型。
- 如果没有显式默认值，使用配置中的第一个模型。
- 随机选择按 `weight` 加权。
- `weight <= 0` 的模型不会参与随机选择。

权重示例：

```yaml
models:
  - name: "fast-model"
    weight: 8
  - name: "slow-but-better-model"
    weight: 2
```

大约 80% 请求会选择 `fast-model`。

## 并发、超时和取消

- HTTP server 有固定读写超时。
- 每个 provider 可以配置 `timeout`，单位秒。
- `OpenAITranslator` 会基于传入 request context 创建带 provider timeout 的子 context。
- 客户端断开或 server shutdown 时，上游请求会收到取消信号。
- `/immersivel` 批量接口默认最多 5 个翻译任务并发。

当前未暴露配置项：

- HTTP server `ReadTimeout` / `WriteTimeout`。
- `/immersivel` 的 `MaxConcurrent`。
- 全局限流中间件。

这些都是后续适合配置化的点。

## 日志

项目有两类日志：

- 标准运行日志：使用 Go 标准库 `log` 输出。
- 翻译审计日志：`logger.TranslationLogger` 写 JSONL 文件，并通过 lumberjack 轮转。

翻译日志字段：

```json
{
  "timestamp": "...",
  "source_text": "...",
  "target_text": "...",
  "source_lang": "EN",
  "target_lang": "ZH",
  "api_url": "...",
  "provider": "openai",
  "model": "...",
  "cache_key": "...",
  "cache_hit": false,
  "process_time_ms": 1234
}
```

注意：翻译日志会记录原文和译文。公网部署或处理敏感内容时，应明确告知用户并设置合适的日志保留策略。

## 安全模型

### 客户端认证

`/deepl/v2/translate` 在 handler 层解析 DeepL v2 风格请求，支持 JSON body 和 `application/x-www-form-urlencoded` form body，将 `text[]` 中的每一项调用 `TranslationService.Translate`，并按输入顺序组装 `translations[]` 响应。请求体限制为 128KiB。`context`、`custom_instructions` 和 `formality` 会合入提示词；glossary、tag handling、translation memory 等 DeepL 高级字段当前只做兼容接收。

`/translate`、`/deepl/v2/translate` 和 `/immersivel` 使用：

```http
Authorization: Bearer <token>
```

也兼容 query token：

```text
/deepl/v2/translate?token=<token>
```

建议文档和生产部署优先使用 Bearer token，因为 query token 容易进入访问日志、浏览器历史和代理日志。

OpenAI-compatible API 使用 `openai.compatible_api.auth_tokens`。

### 上游认证

上游 API key 配置在 provider 的 `api_key` 中，转发时写入：

```http
Authorization: Bearer <api_key>
```

TransBridge 当前不会自动展开环境变量。Kubernetes 或 Docker 生产部署应由部署流程渲染配置文件，或使用外部 Secret 挂载完整配置。

## CI 和 Release

### CI

`.github/workflows/ci.yml` 在 push 和 pull request 上执行：

- `go test ./...`
- `go test -race ./...`
- Docker build

### Release

`.github/workflows/release.yml` 在 `v*` tag 上执行：

- 跑测试。
- 构建 Linux/macOS/Windows 的 amd64/arm64 二进制。
- 打包配置、README、LICENSE 和 docs。
- 创建 GitHub Release。
- 推送 GHCR Docker 镜像：

```text
ghcr.io/<owner>/<repo>:<tag>
ghcr.io/<owner>/<repo>:latest
```

发布示例：

```bash
git add .
git commit -m "release: prepare v0.1.0"
git tag v0.1.0
git push origin main
git push origin v0.1.0
```

## Docker 构建

Dockerfile 使用多阶段构建：

1. `golang:1.22-alpine` 编译静态二进制。
2. `alpine:latest` 作为运行镜像。
3. 从 builder 阶段复制 CA 证书，避免运行阶段访问 apk 仓库。
4. 默认复制 `config.example.yml` 为镜像内 `/app/config.yml`。

默认 GOPROXY：

```text
https://goproxy.cn,direct
```

构建时可覆盖：

```bash
docker build --build-arg GOPROXY=https://proxy.golang.org,direct -t transbridge .
```

`.dockerignore` 会排除 `.env`、`config.yml`、日志和缓存数据库，避免敏感数据进入构建上下文。

## 扩展指南

### 接入新的 OpenAI-compatible 上游

通常无需写代码，只需增加 provider：

```yaml
providers:
  - provider: "openai"
    api_url: "https://example.com/v1/chat/completions"
    api_key: "your-key"
    timeout: 60
    models:
      - name: "model-name"
        weight: 10
```

### 增加新的缓存实现

实现 `cache.Cache` 接口：

```go
type Cache interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string, ttl time.Duration) error
    Clear(ctx context.Context) error
    Close(ctx context.Context) error
}
```

然后在 `main.initCache` 中增加配置解析和实例创建。

### 增加新的 API endpoint

推荐遵循现有分层：

```text
api handler -> service -> translator/cache/logger
```

handler 只处理 HTTP 协议、认证、参数解析和响应格式；业务逻辑放在 service 层。

## 当前限制

- `/translate` 和 `/deepl/v2/translate` 当前不支持请求级指定 provider/model，模型选择由 `ModelManager` 权重决定。
- `/deepl/v2/translate` 不是 DeepL 官方完整实现；glossary、XML/HTML tag handling、translation memory 等高级能力当前不会按 DeepL 原生语义执行。
- `MemoryCache` 不是 LRU，容量满时删除任意一个 map 条目。
- 没有 Prometheus `/metrics` 指标接口。
- RateLimiter 中间件已存在，但未接入配置和默认路由。
- 配置不会自动展开环境变量。
- OpenAI-compatible 代理接口目前不支持流式转发的专门处理。

## 建议路线图

短期优先级：

1. 增加 `/metrics`，暴露请求数、错误数、上游耗时和缓存命中率。
2. 将 HTTP timeout、批量并发数、限流配置写入 YAML。
3. 给 `/translate` 增加可选 `provider` 和 `model` 请求字段，方便调试和精确路由。
4. 增加 OpenAI-compatible streaming 支持。
5. 增加配置校验，启动时提前发现空 token、空模型、非法 cache type 等问题。

中期优先级：

1. 引入结构化运行日志。
2. 支持更多缓存淘汰策略，例如 LRU。
3. 增加管理接口或只读状态接口，展示模型列表、缓存状态和版本信息。
4. 增加更完整的端到端测试，覆盖真实 HTTP handler 行为。
