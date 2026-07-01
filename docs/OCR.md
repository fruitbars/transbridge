# OCR 翻译接口

`POST /ocr/translate` 是给 OCR/文档解析场景的批量翻译接口。相比 `/immersivel`，它接受**每个片段的元素类型和格式**，服务端按类型走不同策略：跳过非语言元素、保留图表编号、拆解 HTML 表格逐格翻译。

## 何时用它

- 输入来自 PP-Structure、LayoutLMv3、MinerU 等版面解析器
- 一次请求含**几十到几百个**混杂片段（段落、标题、表格、页眉页脚、图题）
- 表格是完整的 `<table>` HTML，需要保结构、逐格判断是否翻译

**不需要类型感知的场景走 `/immersivel` 更简单**——它没有 policy 之外的额外过滤。

## 请求

```
POST /ocr/translate
Authorization: Bearer <token with translate scope>
Content-Type: application/json
```

```json
{
  "source_lang": "en",
  "target_lang": "zh",
  "elements": [
    { "id": "p1",  "type": "text",           "content": "This paper introduces..." },
    { "id": "t1",  "type": "title",          "content": "1. Introduction" },
    { "id": "l1",  "type": "list",           "content": "- item A\n- item B", "content_format": "markdown" },
    { "id": "cap", "type": "figure_caption", "content": "Figure 2. Sample distribution" },
    { "id": "tbl", "type": "table",          "content_format": "html",
      "content": "<table>...</table>" },
    { "id": "h",   "type": "header",         "content": "Confidential" },
    { "id": "f",   "type": "footer",         "content": "© 2024 XX Corp" },
    { "id": "e",   "type": "equation",       "content": "E=mc^2" }
  ]
}
```

字段：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `target_lang` | string，必填 | 目标语言，e.g. `zh` / `en` / `ja` |
| `source_lang` | string，可选 | 源语言。可留空让模型自动检测 |
| `elements[]` | array，1..500 | 待处理片段 |
| `elements[].id` | string | 调用方给的稳定 ID，响应里原样返回，便于回填 |
| `elements[].type` | string | 元素类型，见下表 |
| `elements[].content_format` | string，可选 | `text`（默认）、`markdown`、`html` |
| `elements[].content` | string | 元素内容 |

## 元素类型路由

服务端按下表处理。未列出的类型（含空字符串、拼写错误）**退化到 text**。

| type | 处理 | translated | reason（未翻译时） |
| --- | --- | --- | --- |
| `text`, `title`, `list` | 走 policy → cache → model | 是（一般） | `blank` / `policy_kept_original` |
| `table` + `content_format=html` | 拆 `<td>`/`<th>` 逐格翻译，回填保结构 | 至少一格译过为 true | `all_cells_kept_original` / `no_cells` |
| `table` + `content_format=markdown` | 透传给模型，追加"保留 markdown 语法"提示 | 是 | 同上 |
| `table` + `content_format=text` | 视作普通文本 | 是 | 同上 |
| `table_caption`, `figure_caption` | 剥离前置编号（`Figure 2.` / `表 3`），只翻后半再拼回 | 是 | `caption_prefix_only` |
| `header`, `footer` | 直接跳过，原样返回 | 否 | `header_skipped` / `footer_skipped` |
| `figure`, `equation` | 直接跳过 | 否 | `figure_skipped` / `equation_skipped` |
| `reference` | 只翻引号内的文章标题；其他部分（作者/期刊/DOI/卷号/页码）原样保留 | 是（若有引号内容） | `reference_no_translatable_title` / `reference_title_kept_original` |
| 其他 type | 退化到 text | 是（一般） | — |

### 为什么 reference 特殊处理

论文参考文献条目里的**人名、期刊/会议名、年份、卷期号、页码、DOI** 都不该翻译——翻了反而破坏检索。真正值得翻的是**文章标题**。IEEE、GB/T 7714、大多数中文期刊格式的文章标题都**用引号包裹**：

- IEEE：`J. Smith, "A novel approach to translation," IEEE Trans. Comput., vol. 42, ...`
- GB/T 7714：`张三. "一种新颖的翻译方法"[J]. 计算机学报, 2024, ...`

服务端因此只识别引号（ASCII `"..."`、Unicode `"..."`、法文 `«...»`、日文 `「...」`/`『...』`），把引号包裹段送模型，用译文原地替换。作者、期刊名、DOI 等自然不在引号里，被完整保留。

**边界行为**：

- 没引号（例如 APA 格式 `Smith, J. (2024). A novel approach ...`）→ 返回 `reason: reference_no_translatable_title`，不调模型
- 未闭合引号（可能是 OCR 漏识别）→ 忽略该引号，其他段照常处理
- ASCII 单引号（如 `Smith's`）→ **不识别**，避免误吃缩写和所有格

若这套启发式和你的场景冲突（例如你的引用格式恰好不带引号，但你确实需要翻译），把该条目当作 `text` 类型传，让它走完整 policy 管线。

## `content_format` 分支

| format | 处理 | 备注 |
| --- | --- | --- |
| `text`（默认） | 走 policy → cache → model 全套 | 见 [`TRANSLATION.md`](./TRANSLATION.md) |
| `markdown` | 在 `content` 前追加提示句，然后送模型；模型输出剥掉可能回显的提示前缀 | 模型看到的 input 是"The following content is in markdown. Preserve all markdown syntax...\n\n{原内容}"，template 里其他部分不动 |
| `html` | 只有 `type=table` 支持，其他 type 返回 `reason: html_only_supported_for_table` | 表格拆解见下 |

## HTML 表格拆解（type=table + content_format=html）

以下这种输入：

```html
<table>
  <thead><tr><th>Name</th><th>Age</th></tr></thead>
  <tbody>
    <tr><td colspan="2">Employees</td></tr>
    <tr><td>Alice</td><td>30</td></tr>
    <tr><td>Bob</td><td>25</td></tr>
  </tbody>
</table>
```

服务端处理流程：

1. `golang.org/x/net/html` 解析成 DOM，用 `body` 作 context 保持 table 结构完整
2. 深度优先遍历，收集所有 `<td>`/`<th>` 节点及其 innerText（`<br>` → `\n`，多空白折叠为单空格，两端 trim）
3. 每个 cell 文本独立走 `TranslationService.Translate`：
   - `Name` → 命中中文字典 → `名称`
   - `Age` → 送模型 → 译文
   - `30`, `25` → policy skip（数字）→ 原样
   - `Alice`, `Bob` → 送模型 → 译文（真实模型可能保留姓名不译，取决于 prompt）
   - `Employees` → 送模型 → 译文
4. 用译文替换对应 cell 的子节点（清空原有 → 写文本节点 + `<br>`），未变的 cell 保留原节点
5. 序列化整棵 DOM 输出

**保留的**：table 的整体结构（`thead/tbody/tfoot/tr`）、每个 cell 的 `colspan`/`rowspan` 属性、非 td 的兄弟节点、`<br>` 换行、并发限制（默认 5 个 goroutine 处理 cell）

**内联标签**（`<b>` `<i>` `<em>` `<strong>` `<u>` `<sub>` `<sup>` `<code>` `<span>` `<mark>` `<small>` `<a>`）在 cell 里出现时会被 **tokenize** 成 `⟪N⟫...⟪/N⟫` 占位符送模型，翻译后 detokenize 还原。示例：

```
输入：H<sub>2</sub>O + Ca<sup>2+</sup>
送模型：H⟪1⟫2⟪/1⟫O + Ca⟪2⟫2+⟪/2⟫
译文：H⟪1⟫2⟪/1⟫O + Ca⟪2⟫2+⟪/2⟫   （模型保留 placeholder）
输出：H<sub>2</sub>O + Ca<sup>2+</sup>
```

模型看到含 placeholder 的 cell 时，服务端会在 input 前加一句提示："Preserve every ⟪N⟫...⟪/N⟫ marker exactly as-is..."。若模型仍然丢掉某个 placeholder（罕见），detokenize 只输出该 placeholder 内的文本，不留符号残渣。`<a>` 的 `href` 属性通过 record 保留。

**cell 走保守 policy**：表格每个 cell 都走 `TranslateConservative`——不在内置中英文字典里的**短英文单词**（≤12 字符）会直接 skip 保留原文。这样"Alice/Bob/IL/May"之类的姓名列、州名缩写、月份不会被强行翻译。数据表格里典型的字段名（"Name/Age/Total"）能命中字典。段落文本（≥13 字符或含空格的短语）仍会送模型。

**丢失的**（当前折中）：
- 内联标签**跨越 cell 边界**的情形（HTML 里罕见）
- 多层表头的 colspan/rowspan 语义（属性还在，但模型每格看不到跨行/跨列关系）
- 单元格里的 `<img>`：图片本身保留，但 `alt` 属性目前不翻译

## Caption 编号保留

`Figure 2. Sample distribution` 会被切成前缀 `Figure 2. ` 和 rest `Sample distribution`。只有 rest 送模型，返回时前缀原样拼回。识别的模式：

- 英文：`Figure 2` / `Fig. 3` / `Table 4` 后接 `.` 或 `:` 空格
- 中文：`图 2` / `表 3` 后接 `、`、`.` 或 `:`
- 带小数：`Figure 2.1`、`表 3.2`

如果 caption 只有编号没内容（如 `Figure 4.` 单独出现），返回 `reason: caption_prefix_only`，不调模型。

## 响应

```json
{
  "code": 200,
  "translations": [
    {
      "id": "p1",
      "type": "text",
      "content": "本文介绍...",
      "translated": true
    },
    {
      "id": "cap",
      "type": "figure_caption",
      "content": "Figure 2. 样本分布",
      "translated": true
    },
    {
      "id": "tbl",
      "type": "table",
      "content_format": "html",
      "content": "<table>...</table>",
      "translated": true
    },
    {
      "id": "h",
      "type": "header",
      "content": "Confidential",
      "translated": false,
      "reason": "header_skipped"
    },
    {
      "id": "e",
      "type": "equation",
      "content": "E=mc^2",
      "translated": false,
      "reason": "equation_skipped"
    }
  ]
}
```

字段：

| 字段 | 说明 |
| --- | --- |
| `id` | 请求里的 ID，一一对应 |
| `type` | 请求里的 type（未指定则填 `text`） |
| `content_format` | 请求里的 format |
| `content` | **原格式的译文**；未翻译时等于原文 |
| `translated` | 是否至少发生了一次模型翻译 |
| `reason` | 未翻译或部分翻译时的说明，见下表 |
| `error` | 翻译异常时的错误消息（此时 `content` 保留原文） |

### reason 一览

| reason | 触发 |
| --- | --- |
| `already_in_target_lang` | 主导脚本已经是 target 语言（CJK/Latin/Cyrillic/Arabic/Hangul/Kana），跳过 |
| `header_skipped` / `footer_skipped` / `figure_skipped` / `equation_skipped` | 类型被硬编码跳过 |
| `reference_no_translatable_title` | Reference 条目没有引号包裹的文章标题可翻译 |
| `reference_title_kept_original` | Reference 引号内容全部被 policy skip（如全数字/URL） |
| `blank` | content trim 后为空 |
| `caption_prefix_only` | Caption 只有编号 |
| `html_only_supported_for_table` | 非 `table` 类型传 `content_format=html` |
| `policy_kept_original` | 送到 TranslationService 但被 policy skip（详见 [`TRANSLATION.md`](./TRANSLATION.md) 第 2 节） |
| `no_cells` | HTML table 里没找到任何 `<td>`/`<th>` |
| `all_cells_kept_original` | 表格所有 cell 都被 policy skip |

## 并发与限流

- 每个请求内元素间并发：默认 5（`HandlerConfig.MaxConcurrent`）
- 每个表格内 cell 间并发：同一个信号量，共享 5

上游模型的并发和 QPS 限速由 provider 级的 rate limit 处理（见 admin 里的 `provider_rate_limit`），/ocr/translate 不做额外限制。

请求上限 500 个 elements，超过返回 400。

## 与其他接口的关系

- [`TRANSLATION.md`](./TRANSLATION.md) —— 底层 policy / cache / quality gate 通用文档，本接口所有的"是否翻译"决策最终走那里
- `/translate` —— 单条纯文本
- `/deepl/v2/translate` —— DeepL 兼容批量
- `/immersivel` —— 沉浸式翻译批量（无类型感知）
- `/v1/chat/completions` —— OpenAI 兼容代理，不走 policy

## 观察

/ocr/translate 内部每次调用 `TranslationService.Translate` 都会写 admin SQLite 的 `request_log`：

- **一个 element 可能生成多条日志**（尤其是 HTML 表格，每 cell 一条）
- 每条日志的 `endpoint` 字段目前是 provider 的调用端点，不会区分出 `/ocr/translate` —— 如果需要按接口维度统计，后续再加

## 调试日志

配置里加 `ocr.debug_log_path` 后，服务端把每次 `/ocr/translate` 的**完整 request + response + 每 element 的处理 trace** 以 JSONL 追加到指定文件。用于集成方回收生产真实数据供离线分析（policy 决策分布、模型翻译质量、内联标签 round-trip 成功率等）。

```yaml
ocr:
  debug_log_path: "/var/log/transbridge/ocr-debug.jsonl"
  debug_log_max_size_mb: 100    # 默认 100，单文件超过后自动 rotate
  debug_log_max_files: 5        # 保留 5 个历史，最老的删掉
```

**默认关闭**（`debug_log_path` 为空时零开销，request 路径完全不进日志分支）。

一行一条 record：

```json
{
  "ts": "2026-07-01T10:56:23.530151Z",
  "request_id": "ocr-1782903383530153000-1",
  "source_lang": "en", "target_lang": "zh",
  "elapsed_ms": 2, "element_count": 4,
  "request":  { /* 完整 OCRRequest */ },
  "response": { /* 完整 OCRResponse */ },
  "trace": [
    { "id": "h",   "type": "header",    "route": "skip_type",     "reason": "header_skipped" },
    { "id": "zh",  "type": "text",      "route": "language_skip", "reason": "already_in_target_lang" },
    { "id": "t",   "type": "table",     "route": "table_html",    "translated": true,
      "cells_total": 2, "cells_translated": 1, "cells_skipped": 1, "placeholder_count": 1 },
    { "id": "ref", "type": "reference", "route": "reference",     "reason": "reference_no_translatable_title" }
  ]
}
```

**trace.route** 值域：`skip_type` / `reference` / `language_skip` / `table_html` / `caption` / `html_reject` / `markdown` / `text` / `cancelled`。

**表格额外字段**：`cells_total`（去除空 cell 前的总数）、`cells_translated`（真的被翻译的数目）、`cells_skipped`（policy skip + 同语言 skip 加总）、`placeholder_count`（含内联标签的 cell 数）。

**实现细节**：
- 写入是异步（内部 128 大小的 channel + 后台 goroutine），不会阻塞正常翻译请求
- 队列满会静默 drop（`ocr.DebugDropped()` 返回累计丢弃数），优先保证 handler 正常运行
- Rotation 按字节触发：单文件超过 `max_size_mb` 就把 `foo.jsonl` 改名为 `foo.jsonl.1`，历史文件依次后推
- 内容可能含调用方的 OCR 文本，**注意隐私**——路径请落在受控目录并按需清理

## 二版计划

- 数据表格可选 markdown 整表模式（见 [`TRANSLATION.md`](./TRANSLATION.md) 里对合并单元格和结构校验的讨论）
- 请求维度并发独立配置（现在跟表格 cell 抢一个信号量）
- 术语表（glossary）：调用方能提交 `{"neural network": "神经网络"}` 强制映射
- Figure 元素的 `alt` 文本翻译
