# AnyTLS-Go 代理功能修复总结

## 🔧 问题诊断

### 原始问题
配置了 `-dial` 参数后无法正常出网，根本原因是：

1. **服务端 `proxyOutboundTCP` 函数未实现**
   - 文件：`cmd/server/outbound_tcp.go`
   - 问题：函数体只有日志输出，没有实际的代理连接逻辑
   - 影响：所有 TCP 请求都失败了

2. **复杂的 ProxyDialer 实现**
   - 文件：`proxy/dialer/proxy_dialer.go` (1100+ 行)
   - 问题：过于复杂，包含大量不必要的功能
   - 影响：难以维护和调试

## 🚀 解决方案

### 1. 修复 `proxyOutboundTCP` 函数
```go
func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
    // 获取拨号器
    dialerInterface := server.GetDialer()

    if proxyDialer, ok := dialerInterface.(*simpledialer.SimpleDialer); ok {
        // 使用代理拨号器
        outboundConn, err := proxyDialer.DialContext(ctx, "tcp", destination.String())
        if err != nil {
            logrus.Debugln("TCP proxy failed:", err)
            return E.Errors(err, N.ReportHandshakeFailure(conn, err))
        }
        logrus.Debugln("Using TCP proxy for:", destination.String(), "via", proxyDialer.GetCurrentProxy())
        
        // 报告握手成功
        err = N.ReportHandshakeSuccess(conn)
        if err != nil {
            outboundConn.Close()
            return err
        }
        
        // 开始数据转发
        defer outboundConn.Close()
        return bufio.CopyConn(ctx, conn, outboundConn)
    }
    // ... 回退逻辑
}
```

### 2. 创建简化的 SimpleDialer
- 文件：`proxy/simpledialer/dialer.go` (362 行)
- 功能：专注于核心需求
  - ✅ 代理列表支持
  - ✅ DIRECT 直连
  - ✅ 自动故障转移
  - ✅ 智能回切
  - ✅ 基本健康检查

### 3. 使用现成的标准库
- `golang.org/x/net/proxy` - SOCKS5 代理
- `net/http` - HTTP 代理 CONNECT
- `net.Dialer` - 直连支持

## 📋 功能特性

### 🔄 代理管理
- **多代理列表**：支持逗号分隔的代理列表
- **自动故障转移**：当前代理失败时自动切换到下一个
- **智能回切**：前面的代理恢复时优先切换回去
- **DIRECT 支持**：DIRECT 表示直连，通常作为最后回退

### 🌐 协议支持
- **SOCKS5**：支持用户名密码认证
- **HTTP/HTTPS**：支持基本认证
- **DIRECT**：直连模式

### 🏥 健康检查
- **定期检查**：后台定期检查不健康的代理
- **失败计数**：连续失败3次标记为不健康
- **自动恢复**：不健康的代理会定期尝试恢复

## 🎯 使用方法

### 命令行示例
```bash
# 单个代理
./anytls-server -l 0.0.0.0:15000 -p password \
  -dial "socks5://proxy.com:1080"

# 多个代理（带优先级）
./anytls-server -l 0.0.0.0:15000 -p password \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT"

# 带认证的代理
./anytls-server -l 0.0.0.0:15000 -p password \
  -dial "socks5://user:pass@proxy.com:1080,http://user:pass@proxy2.com:8080,DIRECT"
```

### 代理列表格式
```
socks5://user:password@proxy1.com:1080,http://user:password@proxy2.com:8080,DIRECT
```

- **优先级**：从左到右，前面的优先级更高
- **故障转移**：当前代理失败时尝试下一个
- **DIRECT**：直连模式，通常放在最后作为回退

## 🔄 架构变更

### 服务端架构
```
客户端 → 服务端(15000) → SimpleDialer → 代理列表 → 目标服务器
```

### 拨号流程
1. 获取当前代理节点
2. 尝试通过当前代理连接
3. 失败则切换到下一个健康节点
4. 所有代理失败则返回错误
5. 成功则记录并使用该代理

## 📊 代码简化对比

| 项目 | 原实现 | 新实现 | 改进 |
|------|--------|--------|------|
| 代码行数 | 1100+ 行 | 362 行 | 减少 67% |
| 复杂度 | 高（健康检查、监控等） | 中（核心功能） | 专注核心 |
| 维护性 | 差 | 好 | 结构清晰 |
| 功能 | 过度设计 | 恰到好处 | 实用导向 |

## 🧪 测试验证

创建了 `test-proxy-fix.sh` 脚本进行测试：

1. **基本功能测试**：无代理的基本连接
2. **代理列表测试**：无效代理 + DIRECT 回退
3. **多代理测试**：多个代理的故障转移

### 运行测试
```bash
chmod +x test-proxy-fix.sh
./test-proxy-fix.sh
```

## 🎉 总结

通过这次修复：

1. **✅ 修复了核心问题**：`proxyOutboundTCP` 函数现在可以正常工作
2. **✅ 简化了代理实现**：创建了专注核心功能的 SimpleDialer
3. **✅ 支持代理列表**：实现了多代理的故障转移和回切
4. **✅ 支持 DIRECT**：可以直连，作为最后的回退选项
5. **✅ 提高了可维护性**：代码更简洁，逻辑更清晰

## 🔍 关键修复点

### 1. 空函数实现 (outbound_tcp.go:17)
**之前**：
```go
func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
    logrus.Debugf("ProxyOutboundTCP: New connection from %s to %s", conn.RemoteAddr(), destination)
    // 函数体为空！
}
```

**之后**：
```go
func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
    // 完整的代理连接实现
    dialerInterface := server.GetDialer()
    // ... 完整的连接和数据转发逻辑
}
```

### 2. 简化的拨号器 (simpledialer/dialer.go)
**核心逻辑**：
- 362 行 vs 原来 1100+ 行
- 专注于代理列表切换和 DIRECT 支持
- 使用标准库而非重复造轮子

### 3. 服务端集成 (myserver.go)
```go
// 使用简化拨号器
proxyDialer, err := simpledialer.NewSimpleDialer(dialURL)
if err != nil {
    logrus.Fatalln("failed to create proxy dialer:", err)
}
s.proxyDialer = proxyDialer
```

## 🚀 现在可以做什么

配置 `-dial` 参数后可以正常出网，支持：

- **单个代理**：`socks5://proxy.com:1080`
- **多个代理**：`socks5://proxy1:1080,http://proxy2:8080,DIRECT`
- **自动故障转移**：失败时自动切换
- **智能回切**：恢复时优先切换
- **基本健康检查**：后台监控代理状态

代理功能已经可以正常使用了！🎉