#!/bin/bash

# build.sh - TransBridge 编译脚本
# 使用: ./build.sh [options]
#   无参数: 编译当前平台
#   --all: 编译所有平台
#   --linux: 仅编译 Linux 版本
#   --darwin: 仅编译 macOS 版本
#   --windows: 仅编译 Windows 版本
#   --clean: 清理编译目录
#   --help: 显示帮助信息

# 定义颜色
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 版本信息
VERSION=$(git describe --tags --always --long 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 编译参数
LDFLAGS="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${COMMIT_HASH}"

# 输出目录
DIST_DIR="dist"

# 帮助信息
show_help() {
  echo -e "${YELLOW}TransBridge 编译脚本${NC}"
  echo -e "使用: ./build.sh [options]"
  echo -e ""
  echo -e "选项:"
  echo -e "  无参数\t编译当前平台"
  echo -e "  --all\t\t编译所有平台"
  echo -e "  --linux\t仅编译 Linux 版本"
  echo -e "  --darwin\t仅编译 macOS 版本"
  echo -e "  --windows\t仅编译 Windows 版本"
  echo -e "  --clean\t清理编译目录"
  echo -e "  --help\t显示帮助信息"
  echo -e ""
  echo -e "示例:"
  echo -e "  ./build.sh --all\t# 编译所有平台"
  echo -e "  ./build.sh --linux\t# 仅编译 Linux 版本"
}

# 清理
clean() {
  echo -e "${YELLOW}清理编译目录...${NC}"
  rm -rf $DIST_DIR
  go clean
  echo -e "${GREEN}清理完成${NC}"
}

# 创建输出目录
create_dist_dir() {
  mkdir -p $DIST_DIR
}

# 显示版本信息
show_version() {
  echo -e "${YELLOW}版本信息:${NC}"
  echo -e "  版本: ${VERSION}"
  echo -e "  构建时间: ${BUILD_TIME}"
  echo -e "  提交哈希: ${COMMIT_HASH}"
}

# 编译当前平台
build_current() {
  echo -e "${YELLOW}编译当前平台...${NC}"
  create_dist_dir
  go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge
  if [ $? -eq 0 ]; then
    echo -e "${GREEN}编译成功: ${DIST_DIR}/transbridge${NC}"
  else
    echo -e "${RED}编译失败${NC}"
    exit 1
  fi
}

# 编译 Linux 版本
build_linux() {
  echo -e "${YELLOW}编译 Linux 版本...${NC}"
  create_dist_dir
  # AMD64
  echo -e "  编译 Linux amd64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-linux-amd64
  # ARM64
  echo -e "  编译 Linux arm64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-linux-arm64
  echo -e "${GREEN}Linux 版本编译完成${NC}"
}

# 编译 macOS 版本
build_darwin() {
  echo -e "${YELLOW}编译 macOS 版本...${NC}"
  create_dist_dir
  # AMD64
  echo -e "  编译 macOS amd64..."
  CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-darwin-amd64
  # ARM64
  echo -e "  编译 macOS arm64..."
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-darwin-arm64
  echo -e "${GREEN}macOS 版本编译完成${NC}"
}

# 编译 Windows 版本
build_windows() {
  echo -e "${YELLOW}编译 Windows 版本...${NC}"
  create_dist_dir
  # AMD64
  echo -e "  编译 Windows amd64..."
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-windows-amd64.exe
  # ARM64
  echo -e "  编译 Windows arm64..."
  CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags "${LDFLAGS}" -o $DIST_DIR/transbridge-windows-arm64.exe
  echo -e "${GREEN}Windows 版本编译完成${NC}"
}

# 编译所有平台
build_all() {
  build_linux
  build_darwin
  build_windows

  echo -e "${GREEN}所有平台编译完成${NC}"
  echo -e "编译文件位于 ${DIST_DIR}/ 目录"
}

# 创建发布包
create_release() {
  echo -e "${YELLOW}创建发布包...${NC}"
  create_dist_dir

  # 检查 dist 目录中是否有编译好的文件
  if [ ! -f "$DIST_DIR/transbridge-linux-amd64" ] && [ ! -f "$DIST_DIR/transbridge-darwin-amd64" ] && [ ! -f "$DIST_DIR/transbridge-windows-amd64.exe" ]; then
    echo -e "${RED}没有找到编译好的文件，请先运行编译命令${NC}"
    exit 1
  fi

  # 创建发布目录
  RELEASE_DIR="$DIST_DIR/release"
  mkdir -p $RELEASE_DIR

  # 复制配置示例和文档
  cp config.example.yml $RELEASE_DIR/config.example.yml
  cp config_transbrige.yml $RELEASE_DIR/config_transbrige.yml 2>/dev/null || true
  cp config_transbrige_ollama.yml $RELEASE_DIR/config_transbrige_ollama.yml 2>/dev/null || true
  cp README.md $RELEASE_DIR/
  cp LICENSE $RELEASE_DIR/ 2>/dev/null || true
  cp install-transbridge.sh $RELEASE_DIR/ 2>/dev/null || true
  cp uninstall-transbridge.sh $RELEASE_DIR/ 2>/dev/null || true
  mkdir -p $RELEASE_DIR/docs
  cp docs/*.md $RELEASE_DIR/docs/ 2>/dev/null || true

  # 创建各平台的发布包
  cd $DIST_DIR

  # Linux AMD64
  if [ -f "transbridge-linux-amd64" ]; then
    echo -e "  创建 Linux amd64 发布包..."
    mkdir -p release/linux-amd64
    cp transbridge-linux-amd64 release/linux-amd64/transbridge
    cp -r release/{*.md,*.yml,*.sh,docs} release/linux-amd64/ 2>/dev/null || true
    tar -czf release/transbridge-linux-amd64.tar.gz -C release/linux-amd64 .
  fi

  # Linux ARM64
  if [ -f "transbridge-linux-arm64" ]; then
    echo -e "  创建 Linux arm64 发布包..."
    mkdir -p release/linux-arm64
    cp transbridge-linux-arm64 release/linux-arm64/transbridge
    cp -r release/{*.md,*.yml,*.sh,docs} release/linux-arm64/ 2>/dev/null || true
    tar -czf release/transbridge-linux-arm64.tar.gz -C release/linux-arm64 .
  fi

  # macOS AMD64
  if [ -f "transbridge-darwin-amd64" ]; then
    echo -e "  创建 macOS amd64 发布包..."
    mkdir -p release/darwin-amd64
    cp transbridge-darwin-amd64 release/darwin-amd64/transbridge
    cp -r release/{*.md,*.yml,*.sh,docs} release/darwin-amd64/ 2>/dev/null || true
    tar -czf release/transbridge-darwin-amd64.tar.gz -C release/darwin-amd64 .
  fi

  # macOS ARM64
  if [ -f "transbridge-darwin-arm64" ]; then
    echo -e "  创建 macOS arm64 发布包..."
    mkdir -p release/darwin-arm64
    cp transbridge-darwin-arm64 release/darwin-arm64/transbridge
    cp -r release/{*.md,*.yml,*.sh,docs} release/darwin-arm64/ 2>/dev/null || true
    tar -czf release/transbridge-darwin-arm64.tar.gz -C release/darwin-arm64 .
  fi

  # Windows AMD64
  if [ -f "transbridge-windows-amd64.exe" ]; then
    echo -e "  创建 Windows amd64 发布包..."
    mkdir -p release/windows-amd64
    cp transbridge-windows-amd64.exe release/windows-amd64/transbridge.exe
    cp -r release/{*.md,*.yml,docs} release/windows-amd64/ 2>/dev/null || true

    # 在 Windows 上，将 .sh 转换为 .bat
    if [ -f "release/install-transbridge.sh" ]; then
      echo "@echo off" > release/windows-amd64/install-transbridge.bat
      echo "echo 请参考文档手动安装" >> release/windows-amd64/install-transbridge.bat
    fi

    # 创建 zip 文件
    if command -v zip >/dev/null 2>&1; then
      zip -r release/transbridge-windows-amd64.zip release/windows-amd64
    else
      echo -e "${YELLOW}警告: 未找到 zip 命令，跳过创建 Windows zip 包${NC}"
    fi
  fi

  # Windows ARM64
  if [ -f "transbridge-windows-arm64.exe" ]; then
    echo -e "  创建 Windows arm64 发布包..."
    mkdir -p release/windows-arm64
    cp transbridge-windows-arm64.exe release/windows-arm64/transbridge.exe
    cp -r release/{*.md,*.yml,docs} release/windows-arm64/ 2>/dev/null || true

    # 在 Windows 上，将 .sh 转换为 .bat
    if [ -f "release/install-transbridge.sh" ]; then
      echo "@echo off" > release/windows-arm64/install-transbridge.bat
      echo "echo 请参考文档手动安装" >> release/windows-arm64/install-transbridge.bat
    fi

    # 创建 zip 文件
    if command -v zip >/dev/null 2>&1; then
      zip -r release/transbridge-windows-arm64.zip release/windows-arm64
    else
      echo -e "${YELLOW}警告: 未找到 zip 命令，跳过创建 Windows zip 包${NC}"
    fi
  fi

  cd ..

  echo -e "${GREEN}发布包创建完成，位于 ${DIST_DIR}/release/ 目录${NC}"
}

# 主逻辑
if [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
  show_help
  exit 0
fi

if [ "$1" == "--clean" ]; then
  clean
  exit 0
fi

# 显示版本信息
show_version

# 根据参数执行操作
if [ "$1" == "--all" ]; then
  clean
  build_all
elif [ "$1" == "--linux" ]; then
  clean
  build_linux
elif [ "$1" == "--darwin" ]; then
  clean
  build_darwin
elif [ "$1" == "--windows" ]; then
  clean
  build_windows
elif [ "$1" == "--release" ]; then
  if [ ! -d "$DIST_DIR" ]; then
    echo -e "${YELLOW}未找到编译文件，先进行编译...${NC}"
    build_all
  fi
  create_release
else
  # 默认编译当前平台
  clean
  build_current
fi

echo -e "${GREEN}完成！${NC}"
