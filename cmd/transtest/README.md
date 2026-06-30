# transtest

TransBridge API 测试 CLI，四个子命令覆盖烟雾测试、缓存验证、并发压测、黄金集回归。

## 构建

`./build.sh` 会在 `dist/` 下同时产出 `transbridge` 和 `transtest`。也可单独构建：

```bash
go build -o dist/transtest ./cmd/transtest
```

跨平台编译跟随 `build.sh --linux/--darwin/--windows/--all`，会生成对应后缀的 `transtest-<os>-<arch>` 二进制。

## 通用参数

四个子命令共用：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--base URL` | `http://localhost:8080` | TransBridge 服务地址 |
| `--token TOKEN` | 空 | Authorization Bearer Token |
| `--timeout SECS` | `60` | 单请求超时 |
| `--verbose` | false | 打印每次请求的详细日志 |
| `--json` | false | 结构化 JSON 输出（适合 CI 抓取） |

退出码：所有子命令成功返回 0，任一失败返回 1。

## 子命令

### smoke — 烟雾测试

逐个打 4 个端点，每个端点返回非 200 或译文为空都判定失败。适合发布后健康检查或 CI 快速验证。

```bash
./dist/transtest smoke --base http://localhost:8080 --token tr-xxx
./dist/transtest smoke --base http://localhost:8080 --token tr-xxx --verbose
./dist/transtest smoke --base http://localhost:8080 --token tr-xxx --json
```

覆盖端点：
- `POST /translate` — TransBridge 简单接口
- `POST /deepl/v2/translate` — DeepL 兼容接口
- `POST /immersivel` — 沉浸式翻译批量接口
- `POST /v1/chat/completions` — OpenAI 兼容接口

### cache — 缓存命中验证

同一句翻译连续发两次。判定逻辑：第二次 < 100ms 且 < 第一次的 20% 视为命中。可用于确认 bbolt/memory/redis 缓存层接入正确。

```bash
./dist/transtest cache --base http://localhost:8080 --token tr-xxx --verbose
```

输出样例：

```
First request:  202ms
Second request: 0ms
Speedup:        446.0x
✓ Cache likely hit (second request <100ms and <20% of first)
```

### bench — 并发压测

`-d` 时长模式与 `-n` 请求数模式互斥。可选 `--rps` 启用令牌桶限速。

```bash
# 50 并发，30 秒
./dist/transtest bench --base http://localhost:8080 --token tr-xxx -c 50 -d 30s

# 20 并发，共 1000 请求，限速 100 RPS
./dist/transtest bench --base http://localhost:8080 --token tr-xxx -c 20 -n 1000 --rps 100

# 结构化输出
./dist/transtest bench --base http://localhost:8080 --token tr-xxx -c 10 -d 10s --json
```

bench 专用参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `-c N` | `10` | 并发 worker 数 |
| `-d DURATION` | — | 运行时长，如 `30s`、`5m` |
| `-n N` | — | 总请求数 |
| `--rps RATE` | `0`（不限） | 目标 RPS |

输出含 RPS、错误率、avg/min/max 以及 p50/p90/p95/p99 延迟。

### quality — 黄金集回归

按 YAML 用例文件逐条调用 `/translate`，对比期望输出。

```bash
# 首次：以当前模型输出为基线写入 expected 字段
./dist/transtest quality --golden cmd/transtest/testdata/golden.yml --update \
  --base http://localhost:8080 --token tr-xxx

# 回归：失败时展示差异
./dist/transtest quality --golden cmd/transtest/testdata/golden.yml --diff \
  --base http://localhost:8080 --token tr-xxx
```

quality 专用参数：

| 参数 | 默认 | 说明 |
| --- | --- | --- |
| `--golden FILE` | `testdata/golden.yml` | 用例文件路径 |
| `--update` | false | 用实际输出回写 expected |
| `--diff` | false | 失败时打印期望/实际差异 |

## 黄金集 YAML 结构

`cmd/transtest/testdata/golden.yml` 自带 17 条用例：

```yaml
cases:
  - name: "简单问候"
    text: "Hello, how are you?"
    source_lang: "en"
    target_lang: "zh"
    expected: "[ZH]Hello, how are you?"
    category: "translate"

  - name: "版本号"
    text: "v1.2.3"
    source_lang: "en"
    target_lang: "zh"
    expected: "v1.2.3"
    category: "preserve"
```

字段说明：

- `category: translate` — 翻译类用例，期望非空。首次用 `--update` 以模型当前输出作为基线，后续比对识别回归
- `category: preserve` — 应保留原文不翻译的场景（文件名、版本号、URL、化学式、纯数字等）。要求与原文严格相等

## 快捷脚本

仓库提供 `scripts/transtest/` wrapper，省去重复输入 `--base` 和 `--token`：

```bash
cp scripts/transtest/.env.example scripts/transtest/.env
# 编辑 .env 填入 TB_BASE / TB_TOKEN

./scripts/transtest/smoke.sh
./scripts/transtest/cache.sh --verbose
./scripts/transtest/bench.sh                 # 默认 10 并发 30 秒
./scripts/transtest/bench.sh -c 50 -d 1m     # 透传给 transtest
./scripts/transtest/quality.sh --diff
```

临时切换环境无需修改 .env：

```bash
TB_BASE=http://staging:8080 TB_TOKEN=tr-yyy ./scripts/transtest/smoke.sh
```

`.env` 已在 `.gitignore` 中，token 不会入库。

## CI 集成示例

```bash
./build.sh
./dist/transbridge -c config.yml &
sleep 3

./dist/transtest smoke   --base http://localhost:8080 --token $TOKEN --json > smoke.json
./dist/transtest cache   --base http://localhost:8080 --token $TOKEN
./dist/transtest quality --base http://localhost:8080 --token $TOKEN --diff
./dist/transtest bench   --base http://localhost:8080 --token $TOKEN -c 20 -d 30s --json > bench.json
```

每个子命令的非零退出码都可直接作为流水线失败信号。
