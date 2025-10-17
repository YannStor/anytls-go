#!/bin/bash

# AnyTLS-Go 构建脚本
# 支持多平台交叉编译

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目信息
PROJECT_NAME="anytls"
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建目录
BUILD_DIR="bin"
DIST_DIR="dist"

# 清理旧的构建文件
echo -e "${BLUE}清理旧的构建文件...${NC}"
rm -rf "$BUILD_DIR"
rm -rf "$DIST_DIR"
mkdir -p "$BUILD_DIR"
mkdir -p "$DIST_DIR"

# 构建信息
echo -e "${BLUE}构建信息:${NC}"
echo -e "  项目名称: ${GREEN}$PROJECT_NAME${NC}"
echo -e "  版本: ${GREEN}$VERSION${NC}"
echo -e "  构建时间: ${GREEN}$BUILD_TIME${NC}"
echo -e "  Git提交: ${GREEN}$GIT_COMMIT${NC}"
echo ""

# 构建标志
LDFLAGS="-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT -s -w"

# 构建函数
build() {
    local os=$1
    local arch=$2
    local ext=$3

    local output_name="${PROJECT_NAME}-server-${os}-${arch}${ext}"
    local output_path="$BUILD_DIR/$output_name"

    echo -e "${YELLOW}构建 $os/$arch...${NC}"

    GOOS=$os GOARCH=$arch go build \
        -ldflags "$LDFLAGS" \
        -o "$output_path" \
        ./cmd/server

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $output_name 构建成功${NC}"

        # 复制到分发目录
        cp "$output_path" "$DIST_DIR/"

        # 显示文件大小
        if command -v du >/dev/null 2>&1; then
            size=$(du -h "$output_path" | cut -f1)
            echo -e "  文件大小: ${BLUE}$size${NC}"
        fi
    else
        echo -e "${RED}✗ $output_name 构建失败${NC}"
        exit 1
    fi
}

# 检查依赖
echo -e "${BLUE}检查构建依赖...${NC}"
if ! command -v go >/dev/null 2>&1; then
    echo -e "${RED}错误: 未找到 Go 编译器${NC}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "  Go版本: ${GREEN}$GO_VERSION${NC}"

# 下载依赖
echo -e "${BLUE}下载依赖...${NC}"
go mod download
go mod tidy

# 构建目标平台
echo -e "${BLUE}开始构建...${NC}"

# Linux AMD64 (主要目标)
build "linux" "amd64" ""

# Linux ARM64
build "linux" "arm64" ""

# Windows AMD64
build "windows" "amd64" ".exe"

# Windows ARM64
build "windows" "arm64" ".exe"

# macOS AMD64
build "darwin" "amd64" ""

# macOS ARM64 (Apple Silicon)
build "darwin" "arm64" ""

# FreeBSD AMD64
build "freebsd" "amd64" ""

# 创建压缩包
echo -e "${BLUE}创建压缩包...${NC}"
cd "$DIST_DIR"

# 创建 tar.gz 包
echo -e "${YELLOW}创建 tar.gz 包...${NC}"
tar -czf "${PROJECT_NAME}-${VERSION}-linux-amd64.tar.gz" \
    "${PROJECT_NAME}-server-linux-amd64" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

tar -czf "${PROJECT_NAME}-${VERSION}-linux-arm64.tar.gz" \
    "${PROJECT_NAME}-server-linux-arm64" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

tar -czf "${PROJECT_NAME}-${VERSION}-darwin-amd64.tar.gz" \
    "${PROJECT_NAME}-server-darwin-amd64" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

tar -czf "${PROJECT_NAME}-${VERSION}-darwin-arm64.tar.gz" \
    "${PROJECT_NAME}-server-darwin-arm64" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

tar -czf "${PROJECT_NAME}-${VERSION}-freebsd-amd64.tar.gz" \
    "${PROJECT_NAME}-server-freebsd-amd64" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

# 创建 zip 包 (Windows)
echo -e "${YELLOW}创建 zip 包...${NC}"
zip -q "${PROJECT_NAME}-${VERSION}-windows-amd64.zip" \
    "${PROJECT_NAME}-server-windows-amd64.exe" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

zip -q "${PROJECT_NAME}-${VERSION}-windows-arm64.zip" \
    "${PROJECT_NAME}-server-windows-arm64.exe" \
    ../README.md ../PROXY_FEATURE.md ../examples/ 2>/dev/null || true

cd ..

# 生成校验和
echo -e "${BLUE}生成校验和...${NC}"
cd "$DIST_DIR"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum * > sha256sum.txt
    echo -e "${GREEN}✓ SHA256 校验和已生成${NC}"
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 * > sha256sum.txt
    echo -e "${GREEN}✓ SHA256 校验和已生成${NC}"
fi
cd ..

# 显示构建结果
echo ""
echo -e "${GREEN}==================== 构建完成 ====================${NC}"
echo -e "${BLUE}构建文件:${NC}"
ls -la "$BUILD_DIR/"

echo ""
echo -e "${BLUE}分发文件:${NC}"
ls -la "$DIST_DIR/"

echo ""
echo -e "${YELLOW}使用示例:${NC}"
echo -e "# Linux AMD64"
echo -e "./${PROJECT_NAME}-server-linux-amd64 -l 0.0.0.0:8443 -p password"
echo ""
echo -e "# Windows AMD64"
echo -e "./${PROJECT_NAME}-server-windows-amd64.exe -l 0.0.0.0:8443 -p password"
echo ""
echo -e "# macOS ARM64"
echo -e "./${PROJECT_NAME}-server-darwin-arm64 -l 0.0.0.0:8443 -p password"
echo ""

# 代理功能示例
echo -e "${YELLOW}代理功能示例:${NC}"
echo -e "# 使用 SOCKS5 代理"
echo -e "./${PROJECT_NAME}-server-linux-amd64 -l 0.0.0.0:8443 -p password \\"
echo -e "  -dial \"socks5://user:pass@127.0.0.1:1080\""
echo ""
echo -e "# 使用代理列表 (高可用)"
echo -e "./${PROJECT_NAME}-server-linux-amd64 -l 0.0.0.0:8443 -p password \\"
echo -e "  -dial \"socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT\" \\"
echo -e "  -health-urls \"https://cp.cloudflare.com/,https://www.google.com\" \\"
echo -e "  -health-interval 1m -health-threshold 2"
echo ""

echo -e "${GREEN}构建成功! 版本: $VERSION${NC}"
```

现在让我执行构建脚本：
