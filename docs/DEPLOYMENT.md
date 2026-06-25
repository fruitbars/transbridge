# TransBridge 部署指南

本文档提供了 TransBridge 的多种部署方式，包括直接运行、Docker 容器、系统服务等。

## 目录

- [环境要求](#环境要求)
- [直接运行](#直接运行)
- [系统服务部署](#系统服务部署)
- [Docker 部署](#docker-部署)
- [Kubernetes 部署](#kubernetes-部署)
- [反向代理配置](#反向代理配置)
- [性能优化](#性能优化)
- [监控设置](#监控设置)
- [安全建议](#安全建议)

## 环境要求

- 支持的操作系统：Linux, macOS, Windows
- 内存建议：至少 512MB
- 硬盘空间：至少 100MB
- 单机部署推荐使用 bbolt 本地持久化缓存；多实例共享缓存时再使用 Redis

## 直接运行

1. 下载最新的二进制文件或从源码编译：

```bash
# 从源码编译
git clone https://github.com/fruitbars/transbridge.git
cd transbridge
./build.sh

# 或直接下载编译好的二进制文件
```

2. 创建配置文件 `config.yml`：

```bash
cp config.example.yml config.yml
```

最小配置结构如下：

```yaml
server:
  port: 8080

providers:
  - provider: "openai"
    api_url: "https://api.openai.com/v1/chat/completions"
    api_key: "your-api-key"
    timeout: 30
    is_default: true
    models:
      - name: "gpt-3.5-turbo"
        weight: 10
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
    path: "cache/transbridge.db"
    ttl:
      value: "permanent"

transapi:
  tokens:
    - "your-api-key"
```

3. 运行服务：

```bash
./transbridge -config config.yml
```

## 系统服务部署

### Linux (systemd)

1. 创建服务文件：

```bash
sudo vim /etc/systemd/system/transbridge.service
```

2. 添加以下内容：

```ini
[Unit]
Description=TransBridge Translation Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/transbridge
ExecStart=/opt/transbridge/transbridge -config /opt/transbridge/config.yml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

3. 创建目录并移动文件：

```bash
sudo mkdir -p /opt/transbridge
sudo cp transbridge /opt/transbridge/
sudo cp config.yml /opt/transbridge/
```

4. 启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable transbridge
sudo systemctl start transbridge
```

5. 检查状态：

```bash
sudo systemctl status transbridge
```

### 或使用提供的安装脚本

```bash
chmod +x install-transbridge.sh
sudo ./install-transbridge.sh
```

## 🐳 Docker 部署

### 使用 Docker Compose（推荐）

项目提供了完整的 Docker Compose 配置，可以快速部署 TransBridge 服务和 Redis 缓存：

1. 确保已安装 [Docker](https://docs.docker.com/get-docker/) 和 [Docker Compose](https://docs.docker.com/compose/install/)

2. 准备配置文件
```bash
cp config.example.yml config.yml
# 根据需要修改 config.yml 中的 API 地址、模型和密钥
```

3. 启动服务
```bash
docker-compose up -d
```

4. 查看日志
```bash
docker-compose logs -f
```

5. 停止服务
```bash
docker-compose down
```

Docker Compose 配置提供了以下功能：
- 自动构建和启动 TransBridge 服务
- 可选的 Redis 缓存服务：需要时执行 `docker-compose --profile redis up -d`
- 配置文件、日志目录和 bbolt 缓存目录挂载
- 健康检查和自动重启
- 灵活的环境变量配置

### 使用 Docker 构建和运行

也可以直接使用 Docker 命令构建和运行：

```bash
# 构建镜像
docker build -t transbridge .

# 如果默认 Go 模块代理访问慢，可以显式指定 GOPROXY
docker build --build-arg GOPROXY=https://goproxy.cn,direct -t transbridge .

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yml:/app/config.yml \
  -v $(pwd)/logs:/app/logs \
  -v $(pwd)/cache:/app/cache \
  --name transbridge \
  transbridge

# 指定版本信息构建
docker build \
  --build-arg BUILD_VERSION=1.0.0 \
  --build-arg BUILD_DATE=$(date -u +'%Y-%m-%d_%H:%M:%S') \
  --build-arg COMMIT_HASH=$(git rev-parse --short HEAD) \
  -t transbridge:1.0.0 .
```

## Kubernetes 部署

1. 创建配置文件 `transbridge-config.yaml`：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: transbridge-config
data:
  config.yml: |
    server:
      port: 8080
    # 注意：TransBridge 不会自动展开 ${...} 环境变量。
    # 生产环境建议在部署流水线中渲染该配置，或使用 Secret 挂载完整配置文件。
    providers:
      - provider: "openai"
        api_url: "https://api.openai.com/v1/chat/completions"
        api_key: "your-api-key"
        timeout: 30
        is_default: true
        models:
          - name: "gpt-3.5-turbo"
            weight: 10
            max_tokens: 2000
            temperature: 0.3
    cache:
      enabled: true
      types: ["memory", "redis"]
      memory:
        ttl:
          value: "1h"
        max_size: 10000
      redis:
        host: "redis"
        port: 6379
        password: ""
        db: 0
        ttl:
          value: "24h"
    transapi:
      tokens:
        - "your-auth-token"
```

2. 创建部署文件 `transbridge-deployment.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: transbridge
  labels:
    app: transbridge
spec:
  replicas: 2
  selector:
    matchLabels:
      app: transbridge
  template:
    metadata:
      labels:
        app: transbridge
    spec:
      containers:
      - name: transbridge
        image: transbridge:latest
        ports:
        - containerPort: 8080
        volumeMounts:
        - name: config-volume
          mountPath: /app/config.yml
          subPath: config.yml
      volumes:
      - name: config-volume
        configMap:
          name: transbridge-config
```

3. 创建服务文件 `transbridge-service.yaml`：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: transbridge
spec:
  selector:
    app: transbridge
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

4. 应用配置：

```bash
kubectl apply -f transbridge-config.yaml
kubectl apply -f transbridge-deployment.yaml
kubectl apply -f transbridge-service.yaml
```

如果使用 Redis 缓存，需要在集群内额外部署 Redis，并确保配置中的 `redis.host` 可解析。

## 反向代理配置

### Nginx

```nginx
server {
    listen 80;
    server_name translate.yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

使用 HTTPS：

```nginx
server {
    listen 443 ssl;
    server_name translate.yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 性能优化

1. 选择合适的缓存：

单机部署建议使用 `memory + bbolt`，重启后仍保留缓存：

```yaml
cache:
  enabled: true
  types: ["memory", "bbolt"]
  memory:
    ttl:
      value: "1h"
    max_size: 10000
  bbolt:
    path: "cache/transbridge.db"
    ttl:
      value: "permanent"
```

多实例部署建议使用 Redis 共享缓存：

```yaml
cache:
  enabled: true
  types: ["memory", "redis"]
  memory:
    ttl:
      value: "1h"
    max_size: 10000
  redis:
    host: "redis-server"
    port: 6379
    password: "your-password"
    db: 0
    ttl:
      value: "7d"
```

2. 适当增加 API 提供商的并发限制（如果支持）

3. 调整日志级别以减少 I/O 操作

## 监控设置

当前版本提供 `/health` 健康检查接口。生产环境可以先用反向代理、容器编排平台或云监控定期探测该接口。Prometheus 指标端点尚未实现，如需指标监控，建议后续增加请求数、上游耗时、缓存命中率和错误数等指标。

## 安全建议

为确保 TransBridge 的安全部署，请考虑以下建议：

1. **API 密钥管理**
    - 定期轮换 API 密钥
    - 对密钥使用访问控制和权限管理
    - 避免在代码库或公共场所泄露密钥

2. **网络安全**
    - 始终使用 HTTPS 保护传输层
    - 考虑使用 WAF (Web Application Firewall) 防护
    - 限制仅必要的 IP 地址访问 API 服务

3. **日志和审计**
    - 定期查看日志文件寻找异常模式
    - 设置日志轮转和保留策略，避免日志占用过多磁盘空间
    - 考虑将日志发送到集中式日志管理系统

4. **容错和恢复**
    - 设置自动重启服务
    - 实施监控和报警系统
    - 定期备份配置文件

5. **资源限制**
    - 设置服务的 CPU 和内存限制
    - 配置速率限制，防止 API 滥用
    - 考虑设置连接数限制

## 高可用部署

对于需要高可用性的生产环境，推荐以下部署架构：

```
                     ┌───────────────┐
                     │  Load Balancer│
                     └───────┬───────┘
                             │
         ┌───────────────────┴───────────────────┐
         │                                       │
┌────────▼─────────┐                 ┌───────────▼────────┐
│ TransBridge Node 1│                 │ TransBridge Node 2 │
└────────┬─────────┘                 └───────────┬────────┘
         │                                       │
         └───────────────────┬───────────────────┘
                             │
                     ┌───────▼───────┐
                     │ Redis Cluster │
                     └───────────────┘
```

部署步骤：

1. 设置共享的 Redis 缓存集群
2. 部署多个 TransBridge 实例
3. 配置负载均衡器，如 Nginx, HAProxy 或云服务提供商的负载均衡服务
4. 确保所有实例使用相同的配置（除了端口等实例特定配置）

## 故障排除

### 日志分析

查看日志以排查问题：

```bash
# 查看服务日志
journalctl -u transbridge

# 查看应用日志
tail -f /path/to/translation.log
```

### 常见问题

1. **服务无法启动**
    - 检查配置文件语法
    - 确认端口未被占用
    - 检查权限问题

2. **翻译失败**
    - 检查 API 密钥是否有效
    - 确认网络连接到翻译服务提供商
    - 检查请求格式是否正确

3. **缓存不工作**
    - 检查缓存配置
    - 确认 Redis 服务可用（如使用 Redis）
    - 检查内存使用情况

4. **性能问题**
    - 检查 API 提供商的速率限制
    - 考虑增加缓存配置
    - 检查系统资源利用率

## 更新和迁移

### 版本更新

1. 备份当前配置
   ```bash
   cp config.yml config.yml.backup
   ```

2. 停止当前服务
   ```bash
   sudo systemctl stop transbridge
   ```

3. 替换可执行文件
   ```bash
   cp new-transbridge /opt/transbridge/transbridge
   ```

4. 更新配置（如需要）
   ```bash
   cp new-config.yml /opt/transbridge/config.yml
   ```

5. 启动服务
   ```bash
   sudo systemctl start transbridge
   ```

### 数据迁移

如需将服务迁移到新服务器：

1. 在新服务器上安装 TransBridge
2. 复制配置文件
3. 如果使用 bbolt 缓存，复制 `cache/transbridge.db`；如果使用 Redis 缓存，可以考虑迁移 Redis 数据（如有必要）
4. 更新 DNS 记录或负载均衡器配置
5. 验证新服务正常工作后，停止旧服务

## 专业支持

如果您在部署过程中遇到问题，可以：

1. 查阅项目 GitHub Issues
2. 发起 GitHub Discussions 或提交 Issue 说明复现步骤
3. 贡献代码或文档改进

## 进阶使用场景

### 与现有系统集成

TransBridge 可以轻松集成到现有系统中，例如：

1. 作为微服务架构的一部分
2. 为内容管理系统提供翻译能力
3. 为聊天机器人或客服系统提供多语言支持

### 定制开发

TransBridge 设计为易于扩展，如需添加新功能：

1. 接入更多 OpenAI-compatible 上游服务
2. 实现自定义的缓存策略
3. 添加更多的 API 端点

欢迎通过 Issue 和 Pull Request 贡献代码、文档和部署经验。
