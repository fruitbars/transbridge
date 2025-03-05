# TransBridge 🌉

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

TransBridge 是一个强大的翻译 API 代理服务，兼容 DeepL API 接口格式，通过调用各种 AI API 实现高质量的机器翻译功能。它提供了丰富的配置选项、灵活的缓存机制和完善的日志记录，可以作为多种 AI 翻译服务的统一代理。

## 🌟 主要特点

- **多提供商支持**：可配置多个翻译 API 提供商，如 OpenAI、ChatGLM、DeepSeek 等
- **多模型加载均衡**：支持基于权重的模型选择策略
- **多级缓存机制**：灵活配置内存缓存和 Redis 缓存
- **API 兼容**：兼容 DeepL API 接口格式，便于无缝迁移
- **认证安全**：支持 API 密钥认证
- **日志记录**：异步日志系统，支持自动轮转
- **高性能设计**：异步日志、缓存优化等提升性能
- **跨平台**：支持 Linux、macOS 和 Windows

## 🚀 快速开始

### 获取项目
```bash
git clone https://github.com/fruitbars/transbridge.git
cd transbridge
```

### 编译

#### 使用编译脚本 (推荐)

项目提供了便捷的编译脚本 `build.sh`，可以轻松编译各种平台的版本：

```bash
# 添加执行权限
chmod +x build.sh

# 显示帮助信息
./build.sh --help

# 编译当前平台
./build.sh

# 编译所有平台
./build.sh --all

# 只编译 Linux 版本
./build.sh --linux

# 只编译 macOS 版本
./build.sh --darwin  

# 只编译 Windows 版本
./build.sh --windows

# 创建完整发布包
./build.sh --release
```

编译产物会存放在 `dist/` 目录中，发布包位于 `dist/release/` 目录。

#### 手动编译

如果不想使用编译脚本，也可以手动编译：

```bash
# 为当前平台编译
go build -o transbridge

# 为 Linux 编译
GOOS=linux GOARCH=amd64 go build -o transbridge-linux-amd64
```

### 配置
创建配置文件 `config.yml`：
```yaml
server:
  port: 8080

providers:
  - provider: "openai"
    api_url: "https://api.openai.com/v1/chat/completions"
    api_key: "your-api-key-1"
    timeout: 30
    is_default: true
    models:
      - name: "gpt-3.5-turbo"
        weight: 10
        max_tokens: 2000
        temperature: 0.3

cache:
  enabled: true
  types: ["memory"]

  memory:
    ttl:
      value: "1h"
    max_size: 10000

prompt:
  template: "Translate the following text from %s to %s:\n\n%s"

auth:
  tokens:
    - "your-api-key"

log:
  enabled: true
  file_path: "logs/translation.log"
  max_size: 100
  max_age: 30
  max_backups: 10
  queue_size: 1000
```

### 运行
```bash
./transbridge -config config.yml
```

### 使用示例
```bash
curl -X POST "http://localhost:8080/v2/translate" \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "text": "Hello world",
    "source_lang": "EN",
    "target_lang": "ZH"
  }'
```

## 📋 详细文档

- [配置详解](CONFIGURATION.md)
- [API 接口文档](docs/API.md)
- [部署指南](docs/DEPLOYMENT.md)

## 🔧 安装为系统服务

使用提供的脚本来安装为系统服务：

```bash
# 下载可执行文件后执行
chmod +x install-transbridge.sh
sudo ./install-transbridge.sh
```

这将创建一个系统服务，并自动启动。

## 🔄 版本更新

详见 [CHANGELOG.md](CHANGELOG.md)

## 🤝 贡献

欢迎贡献代码或提出建议！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解更多信息。

## 📜 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [go-openai](https://github.com/sashabaranov/go-openai) - OpenAI API 客户端
- [lumberjack](https://github.com/natefinch/lumberjack) - 日志轮转库

## ⚠️ 免责声明

本项目仅供学习和研究之用，请勿用于商业用途。使用本项目时请遵守相关 API 服务提供商的服务条款。