#!/bin/bash

# AnyTLS-Go 清理脚本
# 清理所有构建产物和临时文件

set -e

echo "🧹 清理 AnyTLS-Go 项目..."

# 清理构建目录
echo "清理构建目录..."
rm -rf bin/
rm -rf dist/
rm -rf build/

# 清理测试文件
echo "清理测试文件..."
rm -f coverage.out
rm -f coverage.html

# 清理临时文件
echo "清理临时文件..."
find . -name "*.tmp" -type f -delete
find . -name "*.temp" -type f -delete
find . -name "*.log" -type f -delete
find . -name "*.pid" -type f -delete
find . -name "*.bak" -type f -delete
find . -name "*.backup" -type f -delete

# 清理 IDE 文件
echo "清理 IDE 文件..."
find . -name "*.swp" -type f -delete
find . -name "*.swo" -type f -delete
find . -name "*~" -type f -delete

# 清理 OS 文件
echo "清理系统文件..."
find . -name ".DS_Store" -type f -delete
find . -name "Thumbs.db" -type f -delete

# 清理 Go 缓存（可选）
if [ "$1" = "--deep" ]; then
    echo "深度清理 Go 缓存..."
    go clean -cache
    go clean -modcache
fi

echo "✅ 清理完成！"
echo ""
echo "当前项目状态："
echo "源代码文件: $(find . -name '*.go' | wc -l)"
echo "文档文件: $(find . -name '*.md' | wc -l)"
echo "配置文件: $(find . -name '*.yml' -o -name '*.yaml' -o -name '*.sh' | wc -l)"
echo ""
echo "准备提交的文件："
git status --porcelain | grep -E "^A|^M" | wc -l | xargs echo "修改/新增文件数:"
git status --porcelain | grep -E "^A|^M" | cut -c4-
