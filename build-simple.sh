#!/bin/bash

# AnyTLS-Go 简化构建脚本
set -e

# 项目信息
PROJECT_NAME="anytls"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

# 构建目录
BUILD_DIR="bin"
DIST_DIR="dist"

echo "清理旧的构建文件..."
rm -rf "$BUILD_DIR" "$DIST_DIR"
mkdir -p "$BUILD_DIR" "$DIST_DIR"

echo "构建信息:"
echo "  项目名称: $PROJECT_NAME"
echo "  版本: $VERSION"
echo "  构建时间: $BUILD_TIME"
echo ""

# 构建标志
LDFLAGS="-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -s -w"

# 构建函数
build() {
    local os=$1
    local arch=$2
    local ext=$3
    local output_name="${PROJECT_NAME}-server-${os}-${arch}${ext}"

    echo "构建 $os/$arch..."
    GOOS=$os GOARCH=$arch go build \
        -ldflags "$LDFLAGS" \
        -o "$BUILD_DIR/$output_name" \
        ./cmd/server

    cp "$BUILD_DIR/$output_name" "$DIST_DIR/"
    echo "✓ $output_name 构建成功"
}

# 下载依赖
echo "下载依赖..."
go mod download
go mod tidy

# 构建各平台
echo "开始构建..."
build "linux" "amd64" ""
build "linux" "arm64" ""
build "windows" "amd64" ".exe"
build "darwin" "amd64" ""
build "darwin" "arm64" ""

# 生成校验和
echo "生成校验和..."
cd "$DIST_DIR"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > sha256sum.txt
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 * > sha256sum.txt
fi
cd ..

echo ""
echo "==================== 构建完成 ===================="
echo "构建文件:"
ls -la "$BUILD_DIR/"

echo ""
echo "使用示例:"
echo "./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password"
echo ""
echo "代理功能示例:"
echo "./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password -dial \"socks5://127.0.0.1:1080,DIRECT\""
echo ""
echo "版本信息:"
echo "./anytls-server-linux-amd64 -version"
echo ""
echo "连接超时配置:"
echo "./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password \\"
echo "  -dial \"socks5://proxy.com:1080,DIRECT\" \\"
echo "  -connect-timeout 30s -read-timeout 60s -write-timeout 60s"
echo ""
echo "构建成功! 版本: $VERSION"
