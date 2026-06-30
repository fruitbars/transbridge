# TransBridge 配置指南 ⚙️

## 目录
- [配置文件概述](#配置文件概述)
- [服务器配置](#服务器配置)
- [提供商配置](#提供商配置)
- [缓存配置](#缓存配置)
- [SQLite 存储配置](#sqlite-存储配置)
- [管理后台配置](#管理后台配置)
- [翻译策略与缓存质量门禁](#翻译策略与缓存质量门禁)
- [认证配置](#认证配置)
- [日志配置](#日志配置)
- [完整配置示例](#完整配置示例)

## 配置文件概述

TransBridge 使用 YAML 格式的配置文件。默认配置文件名为 `config.yml`，您也可以通过命令行参数指定其他配置文件：

```bash
./transbridge -config custom_config.yml
```

## 服务器配置

服务器配置部分控制 TransBridge 的基本运行参数。

```yaml
server:
  port: 8080           # 服务监听端口
  host: "0.0.0.0"      # 服务监听地址，默认所有地址
```

## 提供商配置

提供商配置是最核心的部分。当前上游统一使用 OpenAI-compatible `/v1/chat/completions` 协议，因此 `provider` 固定使用 `openai`；OpenAI、Ollama、DeepSeek、ChatGLM、vLLM、LM Studio 等兼容服务通过不同的 `api_url`、`api_key` 和模型名区分。

### OpenAI 配置示例
```yaml
providers:
  - provider: "openai"
    api_url: "https://api.openai.com/v1/chat/completions"  # API 地址
    api_key: "your-api-key"                                # API 密钥
    timeout: 30                                            # 请求超时时间（秒）
    is_default: true                                       # 是否为默认提供商
    rate_limit:                                            # 可选，上游限流
      max_concurrent: 5                                    # 最大并发，0 表示不限制
      qps: 2                                               # 每秒请求数滑动窗口，0 表示不限制
      qpm: 60                                              # 每分钟请求数滑动窗口，0 表示不限制
    models:
      - name: "gpt-3.5-turbo"                             # 模型名称
        weight: 10                                         # 负载均衡权重
        max_tokens: 2000                                   # 最大 token 数
        temperature: 0.3                                   # 温度参数
        rate_limit:                                        # 可选，覆盖 provider rate_limit
          max_concurrent: 2
          qps: 1
          qpm: 30
```

### ChatGLM 配置示例
```yaml
providers:
  - provider: "openai"
    api_url: "http://localhost:8000/v1/chat/completions"
    api_key: "your-api-key"
    timeout: 30
    is_default: false
    models:
      - name: "chatglm3-6b"
        weight: 5
        max_tokens: 2000
        temperature: 0.3
```

### Ollama 配置示例
```yaml
providers:
  - provider: "openai"            # Ollama 使用 OpenAI-compatible 接口
    api_url: "http://localhost:11434/v1/chat/completions"
    api_key: "ollama"
    timeout: 30
    is_default: true
    models:
      - name: "llama2"
        weight: 5
        max_tokens: 2000
        temperature: 0.3
```

### 提供商配置参数说明

| 参数 | 说明 | 默认值 | 是否必填 |
|------|------|--------|----------|
| provider | 提供商类型，当前固定为 `openai` | - | 是 |
| api_url | API 接口地址 | - | 是 |
| api_key | API 密钥 | - | 部分必填 |
| timeout | 请求超时时间（秒） | 30 | 否 |
| is_default | 是否为默认提供商 | false | 否 |
| rate_limit.max_concurrent | 该 provider 默认最大并发上游请求数 | 0 | 否 |
| rate_limit.qps | 该 provider 默认每秒请求数，滑动窗口 | 0 | 否 |
| rate_limit.qpm | 该 provider 默认每分钟请求数，滑动窗口 | 0 | 否 |

### 模型配置参数说明

| 参数 | 说明 | 默认值 | 是否必填 |
|------|------|--------|----------|
| name | 模型名称 | - | 是 |
| weight | 负载均衡权重 | 1 | 否 |
| max_tokens | 最大生成 token 数 | 2000 | 否 |
| temperature | 采样温度 | 0.3 | 否 |
| rate_limit.max_concurrent | 覆盖 provider 的最大并发 | 继承 provider | 否 |
| rate_limit.qps | 覆盖 provider 的每秒请求数 | 继承 provider | 否 |
| rate_limit.qpm | 覆盖 provider 的每分钟请求数 | 继承 provider | 否 |

### 上游限流说明

`rate_limit` 用于控制对上游大模型厂商的调用节奏，避免触发厂商的并发、QPS 或 QPM 限制。

```yaml
providers:
  - provider: "openai"
    api_url: "https://api.example.com/v1/chat/completions"
    api_key: "your-api-key"
    rate_limit:
      max_concurrent: 5
      qps: 2
      qpm: 60
    models:
      - name: "fast-model"
        weight: 10
      - name: "strict-model"
        weight: 1
        rate_limit:
          max_concurrent: 1
          qps: 1
          qpm: 20
```

规则：

- `max_concurrent` 控制同一模型进入上游调用队列的最大并发数，包含正在等待 QPS/QPM 窗口的请求。
- `qps` 和 `qpm` 使用滑动窗口，不是固定整秒/整分钟计数。
- provider 级配置会作为该 provider 下所有模型的默认值。
- model 级配置中大于 0 的字段会覆盖 provider 默认值。
- 所有字段为 0 时表示不限流。
- 缓存命中不会调用上游，因此不会消耗上游限流额度。

## 缓存配置

缓存配置支持内存缓存、Redis 缓存和 bbolt 本地持久化缓存，可以按需组合启用。

### 内存缓存配置
```yaml
cache:
  enabled: true                # 是否启用缓存
  types: ["memory"]           # 缓存类型：memory、redis 或 bbolt
  memory:
    ttl:
      value: "1h"            # 缓存过期时间（支持：30s, 5m, 2h, 1d, 1w, permanent）
    max_size: 10000          # 最大缓存条目数
```

### bbolt 缓存配置
```yaml
cache:
  enabled: true
  types: ["bbolt"]
  bbolt:
    path: "data/transbridge_cache.db" # bbolt 数据库文件路径
    ttl:
      value: "permanent"        # 缓存过期时间
```

### Redis 缓存配置
```yaml
cache:
  enabled: true
  types: ["redis"]
  redis:
    host: "localhost"         # Redis 服务器地址
    port: 6379               # Redis 端口
    password: ""             # Redis 密码
    db: 0                    # Redis 数据库编号
    ttl:
      value: "24h"          # 缓存过期时间
```

### 缓存配置参数说明

| 参数 | 说明 | 默认值 | 是否必填 |
|------|------|--------|----------|
| enabled | 是否启用缓存 | false | 是 |
| types | 缓存类型列表 | [] | 是 |
| ttl.value | 缓存过期时间 | "1h" | 否 |
| max_size | 最大缓存条目数 | 10000 | 否 |
| bbolt.path | bbolt 数据库文件路径 | "data/transbridge_cache.db" | 否 |

types 可以取值 ["memory"]、["redis"]、["bbolt"]、["memory", "bbolt"] 和 ["memory", "redis"]。

## SQLite 存储配置

SQLite 用于管理后台的持久化数据，包括模型配置、访问 token、prompt 版本、请求统计和历史日志。它不替代 bbolt 翻译缓存；两者职责不同：

- `storage.sqlite`：管理数据、统计数据、审计数据，适合查询、分页和迁移。
- `cache.bbolt`：翻译结果 KV 缓存，适合快速命中和 TTL 管理。

```yaml
storage:
  enabled: true
  type: "sqlite"
  path: "data/transbridge.db"
  log_level: "warn"
```

首次启动时，如果 SQLite 中还没有 provider、token 或 prompt，程序会从 YAML 导入初始配置。后续通过后台新增或修改的数据会保存在 SQLite，服务重启后仍然保留。

## 管理后台配置

```yaml
admin:
  enabled: true
  path: "/admin"
  username: "admin"
  password: "change-me"
  local_only: true
```

字段说明：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| enabled | 是否启用 Web 管理后台 | false |
| path | 后台访问路径 | `/admin` |
| username | Basic Auth 用户名，留空则不启用后台登录认证 | 空 |
| password | Basic Auth 密码，留空则不启用后台登录认证 | 空 |
| local_only | 是否只允许 loopback 地址访问后台 | true |

后台启用后，可访问：

```text
http://127.0.0.1:8080/admin/
```

后台支持：

- 查看实时请求统计、缓存命中、平均耗时、启用模型数。
- 添加、更新、删除模型，并热重载模型管理器。
- 管理 translate / openai / all scope 的 token。
- 保存 prompt 版本并切换当前 active prompt。
- 查询最近请求日志和失败信息。

## 翻译策略与缓存质量门禁

翻译服务在调用模型前会先做保守分类：

- 跳过空白、纯数字、百分比、范围、引用编号、URL、email。
- 跳过单字母、短大写缩写、常见单位、缺失值。
- 跳过高置信化学式、离子式、复合单位、物种名、基因/蛋白/样本编号、日期和金额。
- 中文目标语言下，对 `total`、`mean`、`control`、`treatment` 等高置信短词走内置词典。
- 未命中词典的单个英文短词默认保留原文，不直接交给模型。

模型返回结果会在写缓存前经过质量门禁。以下输出不会写入 memory/bbolt/redis 缓存：

- 空输出。
- “请提供文本”“you haven't provided the actual text” 等要求补充上下文的输出。
- “您提供的文本是”“翻译成简体中文为”“the translation is” 等解释性输出。
- 大量短行断裂并包含 `translate`、`provided`、`please` 等助手话术的输出。

缓存 key 新版本包含 `provider + model + source_lang + target_lang + text`，避免不同模型互相污染。系统仍兼容读取旧缓存 key，但旧缓存命中后也会先经过质量门禁，不合格则忽略并重新翻译。

## 认证配置

配置 API 访问认证信息。

```yaml
transapi:
  tokens:                    # API 密钥列表
    - "your-api-key-1"
    - "your-api-key-2"
```

## 日志配置

配置日志记录相关参数。

```yaml
log:
  enabled: true                        # 是否启用日志
  file_path: "logs/translation.log"    # 日志文件路径
  max_size: 100                        # 单个日志文件最大大小（MB）
  max_age: 30                         # 日志文件保留天数
  max_backups: 10                     # 最大备份文件数
  queue_size: 1000                    # 异步日志队列大小
```

## 完整配置示例

下面是一个包含所有主要配置项的完整示例：

```yaml
server:
  port: 8080
  host: "0.0.0.0"

providers:
  - provider: "openai"
    api_url: "https://api.openai.com/v1/chat/completions"
    api_key: "your-openai-key"
    timeout: 30
    is_default: true
    models:
      - name: "gpt-3.5-turbo"
        weight: 10
        max_tokens: 2000
        temperature: 0.3

  - provider: "openai"
    api_url: "http://localhost:11434/v1/chat/completions"
    api_key: "ollama"
    timeout: 30
    is_default: false
    models:
      - name: "llama2"
        weight: 5
        max_tokens: 2000
        temperature: 0.3

cache:
  enabled: true
  types: ["memory", "bbolt"]
  memory:
    ttl:
      value: "1h"
    max_size: 10000
  bbolt:
    path: "data/transbridge_cache.db"
    ttl:
      value: "permanent"
  redis:
    host: "localhost"
    port: 6379
    password: ""
    db: 0
    ttl:
      value: "24h"

transapi:
  tokens:
    - "your-api-key-1"
    - "your-api-key-2"

log:
  enabled: true
  file_path: "logs/translation.log"
  max_size: 100
  max_age: 30
  max_backups: 10
  queue_size: 1000

prompt:
  template: "Translate the following {{source_lang}} content to {{target_lang}}: {{input}}"
```
