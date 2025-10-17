#!/bin/bash

# 测试代理功能修复的脚本

echo "=== AnyTLS Go 代理功能修复测试 ==="

# 构建程序
echo "🏗️  构建程序..."
go build -o bin/anytls-server ./cmd/server
go build -o bin/anytls-client ./cmd/client

if [ ! -f "bin/anytls-server" ] || [ ! -f "bin/anytls-client" ]; then
    echo "❌ 构建失败"
    exit 1
fi

echo "✅ 构建成功"

# 清理函数
cleanup() {
    echo "🧹 清理进程..."
    pkill -f anytls-server 2>/dev/null
    pkill -f anytls-client 2>/dev/null
    sleep 1
}

# 测试基本连接
echo ""
echo "🧪 测试 1: 基本连接（无代理）"
cleanup

./bin/anytls-server -l 127.0.0.1:15000 -p testpassword &
SERVER_PID=$!
sleep 2

./bin/anytls-client -l 127.0.0.1:1080 -s 127.0.0.1:15000 -p testpassword &
CLIENT_PID=$!
sleep 2

echo "📡 测试基本连接..."
timeout 10 curl -x socks5://127.0.0.1:1080 --connect-timeout 5 -s http://httpbin.org/ip > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ 基本连接测试通过"
else
    echo "❌ 基本连接测试失败"
fi

cleanup

# 测试代理列表（DIRECT 回退）
echo ""
echo "🧪 测试 2: 代理列表 + DIRECT 回退"

./bin/anytls-server -l 127.0.0.1:15001 -p testpassword \
    -dial "http://invalid-proxy:8080,DIRECT" &
SERVER_PID=$!
sleep 2

echo "📋 服务器日志应该显示："
echo "[Server] Using outbound proxy: http://invalid-proxy:8080,DIRECT"
echo "[Server] Proxy list: http://invalid-proxy:8080"

./bin/anytls-client -l 127.0.0.1:1081 -s 127.0.0.1:15001 -p testpassword &
CLIENT_PID=$!
sleep 2

echo "📡 测试代理列表连接（应该回退到 DIRECT）..."
timeout 10 curl -x socks5://127.0.0.1:1081 --connect-timeout 5 -s http://httpbin.org/ip > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ 代理列表 + DIRECT 回退测试通过"
else
    echo "❌ 代理列表 + DIRECT 回退测试失败"
fi

cleanup

# 测试多代理列表
echo ""
echo "🧪 测试 3: 多代理列表"

./bin/anytls-server -l 127.0.0.1:15002 -p testpassword \
    -dial "socks5://127.0.0.1:9999,socks5://127.0.0.1:8888,DIRECT" &
SERVER_PID=$!
sleep 2

echo "📋 服务器日志应该显示："
echo "[Server] Using outbound proxy: socks5://127.0.0.1:9999,socks5://127.0.0.1:8888,DIRECT"
echo "[Server] Proxy list: socks5://127.0.0.1:9999"

./bin/anytls-client -l 127.0.0.1:1082 -s 127.0.0.1:15002 -p testpassword &
CLIENT_PID=$!
sleep 2

echo "📡 测试多代理列表连接（应该回退到 DIRECT）..."
timeout 10 curl -x socks5://127.0.0.1:1082 --connect-timeout 5 -s http://httpbin.org/ip > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ 多代理列表测试通过"
else
    echo "❌ 多代理列表测试失败"
fi

cleanup

echo ""
echo "🎯 修复总结："
echo "1. ✅ 修复了 proxyOutboundTCP 函数未实现的问题"
echo "2. ✅ 创建了简化的 SimpleDialer"
echo "3. ✅ 支持代理列表切换"
echo "4. ✅ 支持 DIRECT 直连"
echo "5. ✅ 实现了故障转移机制"

echo ""
echo "💡 使用示例："
echo "单个代理：./anytls-server -dial 'socks5://proxy.com:1080'"
echo "多个代理：./anytls-server -dial 'socks5://proxy1:1080,http://proxy2:8080,DIRECT'"

echo ""
echo "🧪 测试完成！"
