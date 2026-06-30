# TransBridge API 文档

TransBridge 提供 DeepLx 风格翻译接口、沉浸式翻译批量接口、OpenAI 兼容接口和健康检查接口。

## 接口总览

| 路径 | 方法 | 用途 | 请求格式 |
|------|------|------|----------|
| `/translate` | POST | 简化单条翻译接口 | JSON，`text` 为字符串 |
| `/deepl/v2/translate` | POST | DeepL v2 风格翻译接口 | JSON 或 form，`text` 为字符串数组/重复参数 |
| `/immersivel` | POST | 沉浸式翻译批量接口 | JSON，`text_list` 为字符串数组 |
| `/v1/chat/completions` | POST | OpenAI-compatible 代理接口 | OpenAI Chat Completions JSON |
| `/v1/models` | GET | OpenAI-compatible 模型列表 | 无请求体 |
| `/admin/` | GET | 本地管理后台页面 | HTML |
| `/admin/api/*` | GET/POST/PUT/DELETE | 管理后台 JSON API | JSON |
| `/health` | GET | 健康检查 | 无请求体 |

## 简化翻译接口

`/translate` 是项目早期提供的简化翻译接口，适合 DeepLx 风格客户端和简单调用场景。

### 请求

```
POST /translate
```

#### 请求头

| 名称 | 必填 | 描述 |
|------|------|------|
| Authorization | 是 | Bearer 认证，格式为 `Bearer YOUR_API_KEY` |
| Content-Type | 是 | 固定为 `application/json` |

#### 请求体

```json
{
  "text": "Hello world",
  "source_lang": "EN",
  "target_lang": "ZH"
}
```

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| text | 字符串 | 是 | 要翻译的文本 |
| source_lang | 字符串 | 否 | 源语言代码，例如 "EN", "ZH"；为空时由提示词和上游模型处理 |
| target_lang | 字符串 | 是 | 目标语言代码，例如 "EN", "ZH" |

#### 支持的语言代码

| 代码 | 语言 |
|------|------|
| EN | 英语 |
| ZH | 中文 |
| JA | 日语 |
| KO | 韩语 |
| ES | 西班牙语 |
| FR | 法语 |
| DE | 德语 |
| IT | 意大利语 |
| RU | 俄语 |
| PT | 葡萄牙语 |
| NL | 荷兰语 |
| PL | 波兰语 |
| ... | 其他语言 |

### 响应

```json
{
  "code": 200,
  "data": "你好世界",
  "source_lang": "EN",
  "target_lang": "ZH"
}
```

| 字段 | 类型 | 描述 |
|------|------|------|
| code | 数字 | 状态码，200 表示成功 |
| data | 字符串 | 翻译后的文本 |
| source_lang | 字符串 | 源语言代码 |
| target_lang | 字符串 | 目标语言代码 |

### 错误响应

```json
{
  "code": 401,
  "data": "Invalid API key"
}
```

| 状态码 | 描述 |
|------|------|
| 400 | 请求参数错误 |
| 401 | 未授权（API 密钥无效） |
| 500 | 服务器内部错误 |

### 翻译策略说明

`/translate`、`/deepl/v2/translate` 和 `/immersivel` 都会经过通用翻译策略：

- 数字、引用、URL、email、单位、化学式、物种名、基因/蛋白符号等高风险内容会直接保留。
- 中文目标语言下的高置信表格/短词术语会走确定性词典。
- 未命中词典的单个英文短词默认保留，避免模型输出解释性文本。
- 模型输出如果像“请提供要翻译的文本”“you haven't provided the actual text”等助手话术，会返回给调用方但不会写入缓存。

这层策略用于减少 bbolt/Redis 缓存被异常模型输出污染。

### 示例

#### cURL

```bash
curl -X POST "http://localhost:8080/translate" \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello world",
    "source_lang": "EN",
    "target_lang": "ZH"
  }'
```

#### Python

```python
import requests

url = "http://localhost:8080/translate"
headers = {
    "Authorization": "Bearer your-api-key",
    "Content-Type": "application/json"
}
data = {
    "text": "Hello world",
    "source_lang": "EN",
    "target_lang": "ZH"
}

response = requests.post(url, headers=headers, json=data)
print(response.json())
```

#### JavaScript

```javascript
fetch("http://localhost:8080/translate", {
  method: "POST",
  headers: {
    "Authorization": "Bearer your-api-key",
    "Content-Type": "application/json"
  },
  body: JSON.stringify({
    text: "Hello world",
    source_lang: "EN",
    target_lang: "ZH"
  })
})
.then(response => response.json())
.then(data => console.log(data));
```

## 沉浸式翻译批量接口

用于沉浸式翻译等客户端的批量翻译场景。

### 请求

```
POST /immersivel
```

#### 请求体

```json
{
  "source_lang": "ZH",
  "target_lang": "EN",
  "text_list": ["你好", "世界"]
}
```

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| source_lang | 字符串 | 否 | 源语言代码 |
| target_lang | 字符串 | 是 | 目标语言代码 |
| text_list | 字符串数组 | 是 | 待翻译文本列表，最多 50 条 |

#### 请求头

| 名称 | 必填 | 描述 |
|------|------|------|
| Authorization | 是 | Bearer 认证，格式为 `Bearer YOUR_API_KEY` |
| Content-Type | 是 | 固定为 `application/json` |

### 响应

```json
{
  "code": 200,
  "translations": [
    {
      "index": 0,
      "detected_source_lang": "ZH",
      "text": "Hello"
    }
  ]
}
```

## DeepL v2 翻译接口

`/deepl/v2/translate` 按 DeepL v2 `Translate Text` 的主要 JSON 结构实现。它接收 `text` 数组，并按输入顺序返回 `translations` 数组。

### 请求

```
POST /deepl/v2/translate
```

#### 请求头

| 名称 | 必填 | 描述 |
|------|------|------|
| Authorization | 是 | 支持 `DeepL-Auth-Key YOUR_API_KEY`，也兼容 `Bearer YOUR_API_KEY` |
| Content-Type | 是 | 支持 `application/json` 和 `application/x-www-form-urlencoded` |

#### 请求体

JSON 请求：

```json
{
  "text": ["Hello world", "How are you?"],
  "source_lang": "EN",
  "target_lang": "ZH",
  "context": "User-facing UI text",
  "show_billed_characters": true,
  "formality": "default"
}
```

Form 请求也兼容，`text` 可以重复出现：

```bash
curl -X POST "http://localhost:8080/deepl/v2/translate" \
  -H "Authorization: DeepL-Auth-Key your-api-key" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "text=Hello world" \
  --data-urlencode "text=How are you?" \
  --data-urlencode "source_lang=EN" \
  --data-urlencode "target_lang=ZH"
```

请求体大小限制为 128KiB。

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| text | 字符串数组 | 是 | 待翻译文本，逐条独立翻译并按顺序返回 |
| target_lang | 字符串 | 是 | 目标语言代码 |
| source_lang | 字符串 | 否 | 源语言代码，省略时由上游模型处理 |
| context | 字符串 | 否 | 附加上下文，会合入提示词但不作为待翻译文本 |
| show_billed_characters | 布尔值 | 否 | 为 true 时返回每条输入的字符数估算 |
| formality | 字符串 | 否 | 会作为提示词偏好传给上游模型 |
| custom_instructions | 字符串数组 | 否 | 最多 10 条，每条最多 300 字符，会合入提示词 |

以下 DeepL 字段当前会被兼容接收，但不会直接实现 DeepL 原生语义：`split_sentences`、`preserve_formatting`、`model_type`、`glossary_id`、`glossary_ids`、`style_id`、`translation_memory_id`、`translation_memory_threshold`、`tag_handling`、`tag_handling_version`、`outline_detection`、`enable_beta_languages`、`non_splitting_tags`、`splitting_tags`、`ignore_tags`。

### 响应

```json
{
  "translations": [
    {
      "detected_source_language": "EN",
      "text": "你好，世界",
      "billed_characters": 11
    },
    {
      "detected_source_language": "EN",
      "text": "你好吗？",
      "billed_characters": 12
    }
  ]
}
```

### 错误响应

```json
{
  "message": "Invalid API key"
}
```

## OpenAI 兼容接口

TransBridge 还提供与 OpenAI API 兼容的代理接口。模型名称使用 `/v1/models` 返回的 `provider/model` 格式，例如 `openai/gpt-4o-mini` 或 `openai/llama3.1:latest`。

### 请求

```
POST /v1/chat/completions
```

#### 请求头

| 名称 | 必填 | 描述 |
|------|------|------|
| Authorization | 是 | Bearer 认证，格式为 `Bearer YOUR_API_KEY` |
| Content-Type | 是 | 固定为 `application/json` |

#### 请求体

```json
{
  "model": "openai/gpt-3.5-turbo",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello!"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 100
}
```

| 参数 | 类型 | 必填 | 描述 |
|------|------|------|------|
| model | 字符串 | 是 | 模型名称，格式为 "provider/model" |
| messages | 数组 | 是 | 消息数组，包含多个消息对象 |
| temperature | 浮点数 | 否 | 温度参数，控制生成文本的随机性 |
| max_tokens | 整数 | 否 | 最大输出 token 数量 |

### 模型列表

```
GET /v1/models
```

返回当前配置中可用的模型列表。

### 错误响应

OpenAI 兼容接口会按 OpenAI 风格返回错误对象。请求不存在的模型会返回 `400 invalid_model`，不会静默切换到默认模型。

### 响应

响应格式与 OpenAI API 保持一致：

```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "created": 1677858242,
  "model": "openai/gpt-3.5-turbo",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ]
}
```

## 管理后台接口

管理后台默认挂载在 `/admin/`，需要启用 `admin.enabled` 和 SQLite storage。页面接口使用 Basic Auth，认证账号来自 `admin.username` / `admin.password`。如果 `admin.local_only: true`，只允许 loopback 地址访问。

### 页面

```text
GET /admin/
```

返回内置 HTML 管理页面，支持查看状态、管理模型、管理 token、管理 prompt 版本和查询历史日志。

### JSON API

| 路径 | 方法 | 用途 |
|------|------|------|
| `/admin/api/stats` | GET | 请求数、成功失败、缓存命中、平均耗时、模型/token/prompt 数量 |
| `/admin/api/models` | GET | 列出 SQLite 中的模型和 provider 信息 |
| `/admin/api/models` | POST | 新增或更新模型，成功后热重载 |
| `/admin/api/models?id=1` | DELETE | 删除模型，成功后热重载 |
| `/admin/api/tokens` | GET | 列出 token |
| `/admin/api/tokens` | POST | 新增 token |
| `/admin/api/tokens` | PUT | 更新 token |
| `/admin/api/tokens?id=1` | DELETE | 删除 token |
| `/admin/api/prompts` | GET | 列出 prompt 版本 |
| `/admin/api/prompts` | POST | 新增 prompt 版本 |
| `/admin/api/prompts/activate?id=1` | POST | 启用指定 prompt |
| `/admin/api/logs?limit=100` | GET | 查询最近请求日志 |

新增模型示例：

```json
{
  "provider": "openai",
  "api_url": "http://localhost:11434/v1/chat/completions",
  "api_key": "ollama",
  "provider_timeout": 60,
  "is_default": true,
  "name": "llama3.1:latest",
  "weight": 10,
  "max_tokens": 2000,
  "temperature": 0.3,
  "enabled": true
}
```

新增 token 示例：

```json
{
  "name": "immersive translate",
  "token": "tr-local-token",
  "scope": "translate"
}
```

`scope` 可取：

- `translate`：用于 `/translate`、`/deepl/v2/translate`、`/immersivel`。
- `openai`：用于 `/v1/chat/completions` 和 `/v1/models`。
- `all`：同时允许翻译接口和 OpenAI-compatible 接口。

## 健康检查接口

### 请求

```
GET /health
```

### 响应

```
OK
```

状态码 200 表示服务正常运行。
