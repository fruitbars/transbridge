# 翻译内部机制

本文档解释 TransBridge 收到一次翻译请求之后，如何决定"要不要翻译、由谁翻译、翻译结果是否缓存"。所有描述均对应 `service/translation.go`、`service/policy.go`、`service/quality.go`、`cache/*` 的实际代码，改动这些逻辑时请同步更新本文。

## 1. 端到端时序

```
HTTP 请求 (/translate | /deepl/v2/translate | /immersivel | /v1/chat/completions)
    │
    ▼  ①  参数校验 + Token 鉴权（handler 层）
    │
    ▼  ②  translation policy —— 判定：跳过 | 词典 | 送模型
    │        ├─ skip：直接返回原文
    │        └─ dictionary：查表 → 直接返回译文
    │
    ▼  ③  选定 translator（指定 provider/model 或加权随机）
    │
    ▼  ④  多级缓存查询（memory → bbolt → redis 顺序）
    │        └─ hit：回填上层缓存 → 直接返回
    │
    ▼  ⑤  调用上游 OpenAI-compatible /chat/completions
    │
    ▼  ⑥  quality gate —— 判定这次的模型输出能否入缓存
    │
    ▼  ⑦  返回译文 & 记录 translation log + request log
```

第 ②、④、⑥ 三层决策是本文的核心。

## 2. 是否翻译的策略（policy）

代码在 `service/policy.go`。三种决策：

| Decision | 处理 | 触发条件 |
|---|---|---|
| `skip` | 原样返回，不打上游，不写缓存 | 命中"非自然语言"规则集 |
| `dictionary` | 从内置词典取译文，不打上游 | 中文目标语言 + 命中常见词典 |
| `model` | 交给上游模型 | 其他情况 |

### 2.1 skip：非自然语言片段

`shouldSkipTranslationText()` 按顺序检查以下模式，任一命中即 skip：

1. **引用/编号** —— `[7]`、`[1,2,3]`、`12-18`
2. **纯数字/范围** —— `95.99%`、`±0.5`、`3.14`、`1,234.56`
3. **URL / Email** —— `https://...`、`www.example.com`、`a@b.com`
4. **物种缩写** —— `P. radiata`、`E. coli`（大写字母加点加小写词）
5. **化学式** —— `H2O`、`CaCO3`、`NaCl·2H2O`
6. **带电离子** —— `NO3-`、`Fe3+`、`Cu2+`
7. **复合单位** —— `mg/L`、`kg m2`、`µmol/L`。必须含 `/`、`.`、`·` 分隔符 **或** 空格分隔的每段末尾有数字/正负号（否则纯字母序列会被误判为单位，这是 2026-07-01 修复的 bug，见 `TestTranslationPolicyDoesNotSkipEnglishPhrases`）
8. **基因/蛋白编号** —— `BRCA1`、`p53`、`IL-6`、`ATCC 12345`
9. **日期/财报/货币** —— `2024-03-01`、`Q3`、`FY2023`、`$1,200`、`USD 500`
10. **罗马数字** —— `II`、`VIII`、`XIV`
11. **单单位词汇表** —— `kg` `mg` `mol` `hz` 等常见 SI 单位单独出现
12. **缺失值标记** —— `NA`、`N/A`、`nil`、`null`、`-`、`--`
13. **单字符** —— 任意 1-rune 文本
14. **全大写缩写** —— 2-5 位大写字母（`API`、`HTTP`）
15. **纯符号** —— 无字母无数字（`***`、`---`、`==`）

命中 skip 时：
- **原样返回**，不打模型
- 保留 `logTranslation` 和 `logRequest` 记录，`provider` 字段写入 `"skip"`，`model` 字段写入具体命中的原因，便于审计
- 不写任何缓存层

### 2.2 dictionary：中文常见词直查

`translateCommonTerm()` 只有目标语言为 `zh` / `cn` / `zh-*` 时启用。词典 `zhCommonTerms` 现有 30 余项，覆盖数据分析常见字段名（`text→文本`、`total→总计`、`max→最大值`、`yes→是`、`treatment→处理`…）。

命中时：
- 直接返回词典译文
- 用 `preserveOuterSpace` 保留原文首尾空白（例如 `" total "` → `" 总计 "`）
- 同 skip：不打模型，不写缓存，记审计日志（`provider=dictionary`）

### 2.3 model：交给上游

进入这一步意味着文本是"看起来像自然语言"的内容，进入后续的缓存查询和模型调用。

### 2.4 policy mode（保守模式）

`policyModeConservative` 会额外把 `^[A-Za-z]+$` 且长度 ≤ 12 的单个 ASCII 单词也判成 skip（例如未在词典里的 `mucosa`）。当前默认走 `policyModeGeneral`，保守模式暂未在生产路径启用，保留给后续按调用来源分级用。

## 3. 缓存策略

代码在 `cache/*.go` 和 `service/translation.go` 里的缓存分支。

### 3.1 多级缓存结构

`config_transbrige.yml` 里 `cache.types` 决定启用哪些层，按数组顺序**逐级查找**、**逐级回填**：

```yaml
cache:
  enabled: true
  types: ["memory", "bbolt"]      # 顺序即优先级
  memory: { ttl: { value: "permanent" }, max_size: 100000000 }
  bbolt:  { path: "data/transbridge_cache.db", ttl: { value: "permanent" } }
  redis:  { host: "...", port: 6379, ttl: { value: "permanent" } }
```

行为（`cache/multi.go`）：

- **Get**：从第 0 层开始查，命中则**回填所有比它更浅的层**（浅层写入用其自己的默认 TTL），未命中就问下一层
- **Set**：写入所有层
- **Clear**：清所有层

推荐配置：
- 单机开发：`["memory"]` —— 快、简单
- 单机生产：`["memory", "bbolt"]` —— 内存热点 + 磁盘持久化，重启不丢
- 集群生产：`["memory", "redis"]` —— 本地热点 + 集群共享

### 3.2 缓存键设计

有两套 key，见 `internal/utils/utils.go`：

| Key | 组成 | 前缀 |
|---|---|---|
| **model-scoped**（当前） | `md5(provider:model:source:target:text)` | `transbridge:v2:` |
| **legacy**（兼容读） | `md5(source:target:text)` | `transbridge:` 或 `transbrige:`（历史 typo） |

写入只写 model-scoped。读取按 model-scoped → legacy → 老 typo legacy 依次尝试，找到就命中并回填。这样：

- **同一句话不同模型不复用**（避免 GPT-4 的输出被回填给 Qwen 的 key，反之亦然）
- **升级前的旧缓存仍能命中**（读兼容），但不会被新写入污染
- **provider 有多个上游时**，即使 model 名字相同，key 也不同（因为 provider 前缀不一样）

### 3.3 TTL 语义

TTL 值支持字面量：`30s`、`5m`、`2h`、`1d`、`1w`、`permanent`。翻译内容通常不会过期（一句话翻译结果的正确性不随时间衰减），实践中三层都建议设 `permanent`。

`Set` 传入的 `ttl` 参数：
- `< 0` 或后端配置 `permanent: true` → 永不过期
- `= 0` → 使用后端 `DefaultTTL`（memory 默认 1h，bbolt 默认 24h，redis 默认 24h）
- `> 0` → 用指定值

`service/translation.go` 里 `s.cache.Set(ctx, key, ..., 0)` —— **翻译服务总是传 0**，让缓存后端按自己的默认或永久策略决定，避免硬编码 24h。

### 3.4 memory 层的容量与淘汰

`MemoryCache` 是简单 map + RWMutex，容量达到 `max_size` 后**随机删一个键**（不是 LRU）。默认 `max_size: 10000000`。翻译场景写入频率低、读多写少，随机淘汰的次优表现可接受；如果要严格 LRU，需要引入 `container/list`，暂未做。

后台 goroutine 每分钟扫一次过期项。设置了 `permanent: true` 时**不启动**扫描 goroutine。

### 3.5 bbolt 层

单文件 KV，路径由 `bbolt.path` 指定，默认 `data/transbridge_cache.db`（历史上放在 `cache/` 目录里，2026-06 起改到 `data/`，见 `cache/bbolt.go` 里的默认值）。适合单机长期持久化。

### 3.6 redis 层

`go-redis/v8`，标准 host:port + password + db 配置。集群部署时共享译文可以显著降低重复请求成本。

## 4. Quality Gate（结果能否入缓存）

代码在 `service/quality.go`。**成功调用上游之后**，用 `shouldCacheTranslation(source, translated)` 判定要不要写缓存。返回 `false` 就跳过写缓存但仍然把译文返回给用户。

为什么要这层？大模型偶尔会返回"我无法翻译"、"这看起来像专有名词"之类的元回复。如果盲目缓存，之后同样的输入永远拿到这个垃圾结果。

### 4.1 显式拒绝模式

以下 pattern（不完全大小写敏感）命中即拒缓存：

**英文**
- `you haven't provided...`
- `please paste/provide the actual text/content`
- `I can't/cannot translate`
- `there is no text to translate`
- `not performing translation`
- `looks like a proper noun/title/heading`
- `auto-detected language`
- `the translation is`
- `I'd be happy to help`
- `therefore` / `meaning of`

**中文**
- `您提供的文本` / `您要求翻译` / `意思是` / `可以翻译为` / `翻译成简体中文为`

### 4.2 结构性拒绝

`looksLikeFragmentedAssistantMessage` 检测"多段短行 + 含解释性词汇"的结构：

- 非空行 ≥ 8
- 短行（≤ 12 rune）占比 ≥ 70%
- 归一化后包含 `translate` / `provided` / `please`

这种是模型把回复拆成一堆碎句解释"我不翻译"，直接拒缓存。

### 4.3 归一化

`normalizeQualityText` 会先把 `\r\n` → `\n`、`-\n` → `''`（合并断行连字符）、多空白 → 单空格，再送去 pattern 匹配。这样"you \n haven't \n provided"和"you haven't provided"等价。

## 5. 审计日志

不管走哪条路径（skip / dictionary / cache-hit / model / model-failed），都会写两份日志：

- `translation.log` —— 结构化 JSON，按天轮转（`logger/translation_logger.go`），字段包含 source/target 语言、原文、译文、模型、缓存 key、是否命中、耗时
- SQLite `request_log` —— 通过 admin 控制台"历史日志"查看，字段包含 timestamp / endpoint / provider / model / cache_hit / success / error / process_time_ms / source_chars / target_chars

**source_text 不入库**（只写 chars 数），避免 PII/敏感文本长期落盘。想看原文去 `translation.log`。

## 6. 如何观测各层生效

启用 admin 后，翻译一次并到"历史日志"看：

| 现象 | 含义 |
|---|---|
| `provider=skip`, `cache_hit=false` | policy skip |
| `provider=dictionary`, `cache_hit=false` | 词典命中 |
| `provider=openai model=gpt-...`, `cache_hit=true` | 缓存命中，`process_time_ms` 一般 < 5 |
| 同上但 `cache_hit=false` | 真调了上游，`process_time_ms` 通常 500-3000 |

用 `transtest cache` 子命令做端到端的缓存烟雾测试：

```bash
./dist/transtest cache --base http://localhost:8080 --token tr-xxx --verbose
```

判定标准：第二次请求 <100ms 且不超过第一次的 20%。

## 7. 修改指南（避免踩坑）

- **加 skip 规则**：先补一个 `service/policy_test.go` 用例证明它在你打算跳过的样本上返回 skip，且**在正常英文短句上不返回 skip**。`compoundUnitPattern` 就是因为缺后一半测试而误吃了 `I am not good`（`TestTranslationPolicyDoesNotSkipEnglishPhrases` 就是为此加的回归）
- **加词典项**：只加真的"读音语义都固定"的字段名（表格标题、状态词）。**别加带上下文歧义的词**，那些应该交给模型
- **改缓存 key**：改了就是缓存全废。要不换前缀（`transbridge:v3:`）保留 v2 作为兼容读，要不接受重启后一次 cold cache
- **改 quality gate**：新增 pattern 前先在 `translation.log` 里搜真实的垃圾输出，用最少的表达式覆盖它。别用宽泛的正则，会误伤正常译文
- **改 TTL**：现在服务层传 0，全交给后端默认值。如果要按 provider/model 差异化 TTL，改 `TranslationService.Translate` 里 `s.cache.Set(..., 0)` 那一行
