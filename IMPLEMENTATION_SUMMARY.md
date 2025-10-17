# AnyTLS-Go 代理功能实现总结

## 🎯 功能概述

为 anytls-go 服务器端实现了完整的出站代理功能，支持代理列表、智能故障转移、动态回退和健康监测。

## ✨ 核心功能

### 1. 代理列表支持
- **多代理配置**: 支持逗号分隔的多个代理URL
- **代理协议**: 支持 SOCKS5、HTTP、HTTPS 和 DIRECT（直连）
- **优先级管理**: 按配置顺序优先使用前面的代理

### 2. DIRECT 特殊代理类型
- **直连集成**: 将直连作为特殊的代理类型纳入统一管理
- **自动添加**: `-dialfallback` 等价于在代理列表末尾添加 `DIRECT`
- **统一处理**: 直连和代理享受相同的健康检查和故障转移机制

### 3. 智能故障转移
- **自动切换**: 当前代理失败时自动切换到下一个可用代理
- **优先级回切**: 前面的代理恢复时优先回切到更靠前的代理
- **故障恢复**: 定期检查失败代理的恢复状态

### 4. 数据传输感知
- **实时监控**: 监控实际数据传输的成功/失败
- **智能跳过**: 有数据传输成功时跳过不必要的健康检查
- **即时响应**: 数据传输失败立即标记代理为不健康

### 5. 健康检查机制
- **自定义URL**: 支持自定义健康检查URL列表
- **可配置参数**: 检查间隔、超时时间、成功阈值均可配置
- **多URL验证**: 支持多个检测URL，提高可靠性

### 6. UDP 支持
- **SOCKS5 UDP**: 完整支持 SOCKS5 UDP ASSOCIATE
- **自动回退**: UDP 代理失败时回退到本地UDP
- **协议适配**: 自动处理 SOCKS5 UDP 包格式

## 📋 命令行参数

### 基础代理参数
```bash
-dial "socks5://user:pass@127.0.0.1:1080,http://user:pass@127.0.0.1:8080,DIRECT"
-dialfallback  # 等价于在末尾添加 DIRECT
```

### 健康检查参数
```bash
-health-urls "https://cp.cloudflare.com/,https://www.google.com"
-health-interval 30s    # 检查间隔
-health-timeout 10s     # 超时时间
-health-threshold 1     # 成功阈值
-data-idle 5m          # 数据传输空闲时间
```

## 🔄 工作流程

### 1. 代理选择策略
1. 按配置顺序检查代理健康状态
2. 优先使用最靠前的健康代理
3. 当前代理失败时切换到下一个
4. 前面代理恢复时优先回切

### 2. 数据传输监控
1. 包装所有连接以监控数据传输
2. 成功传输时更新代理健康状态
3. 传输失败时立即标记代理为不健康
4. 基于传输成功时间智能跳过健康检查

### 3. 健康检查机制
1. 定期检查所有代理节点的健康状态
2. 通过HTTP/HTTPS请求验证代理可用性
3. 支持多URL检查，提高准确性
4. 动态调整代理优先级

## 🎮 使用示例

### 基础配置
```bash
# 单个代理
./anytls-server -l 0.0.0.0:8443 -p password -dial "socks5://127.0.0.1:1080"

# 代理列表
./anytls-server -l 0.0.0.0:8443 -p password -dial "socks5://127.0.0.1:1080,http://127.0.0.1:8080,DIRECT"
```

### 高可用配置
```bash
./anytls-server \
  -l 0.0.0.0:8443 \
  -p password \
  -dial "socks5://proxy1.com:1080,socks5://proxy2.com:1080,http://proxy3.com:8080,DIRECT" \
  -dialfallback \
  -health-urls "https://cp.cloudflare.com/,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 15s \
  -health-threshold 2 \
  -data-idle 10m
```

## 🧪 测试覆盖

### 单元测试（12个测试用例）
- ✅ 代理拨号器创建测试（8个子用例）
- ✅ 多协议支持测试（TCP/UDP/IPv4/IPv6）
- ✅ 认证功能测试
- ✅ 代理列表和故障转移测试
- ✅ 智能回切测试
- ✅ DIRECT 连接测试
- ✅ 健康检查配置测试
- ✅ 数据传输感知测试

### 测试运行
```bash
go test ./proxy/dialer -v
```

## 🏗️ 架构设计

### 核心组件
1. **ProxyDialer**: 主代理拨号器，管理代理列表
2. **ProxyNode**: 单个代理节点，包含状态和配置
3. **MonitoredConn**: 连接包装器，监控数据传输
4. **HealthChecker**: 健康检查器，验证代理可用性

### 关键特性
- **线程安全**: 使用读写锁保护并发访问
- **内存高效**: 避免不必要的连接和检查
- **可扩展性**: 易于添加新的代理协议
- **向后兼容**: 完全兼容原有配置

## 📊 性能优化

### 1. 智能跳过机制
- 数据传输成功时跳过健康检查
- 基于空闲时间动态调整检查频率

### 2. 连接复用
- 包装现有连接而非创建新连接
- 保持连接的原有特性

### 3. 故障快速检测
- 数据传输失败立即响应
- 避免等待超时再切换

## 🛡️ 安全特性

### 1. 认证支持
- SOCKS5 用户名/密码认证
- HTTP Basic Authentication
- URL编码处理特殊字符

### 2. 错误处理
- 完善的错误日志记录
- 优雅的降级处理
- 防止敏感信息泄露

## 📈 监控和调试

### 代理状态查询
```go
// 获取当前代理状态
status := dialer.GetProxyStatus()

// 获取所有代理健康状态
healthList := dialer.GetAllProxyHealth()

// 获取当前代理URL
currentURL := dialer.GetCurrentProxyURL()
```

### 日志输出
```
[Server] Using outbound proxy: socks5://user:***@127.0.0.1:1080
[Server] Proxy fallback enabled
[Server] Health check: 4 URLs, interval: 30s, timeout: 10s, threshold: 1, data-idle: 5m
[Server] Proxy marked as unhealthy after 3 consecutive failures
[Server] Proxy health check passed, resuming proxy usage
```

## 🔄 配置迁移

### 从旧版本升级
```bash
# 旧配置
./anytls-server -dial "socks5://127.0.0.1:1080" -dialfallback

# 新配置（等价）
./anytls-server -dial "socks5://127.0.0.1:1080,DIRECT"
```

### 推荐配置
```bash
# 高可用配置（推荐）
./anytls-server \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT" \
  -health-urls "https://cp.cloudflare.com/,https://www.google.com" \
  -health-interval 1m \
  -health-threshold 2
```

## 🎯 总结

本实现为 anytls-go 提供了企业级的代理功能，具备：
- **高可用性**: 多代理 + 智能故障转移
- **高性能**: 数据传输感知 + 智能跳过
- **高可靠性**: 完善的健康检查机制
- **高兼容性**: 支持多种代理协议
- **高可维护性**: 丰富的监控和调试功能

通过这些功能，anytls-go 现在可以在复杂的网络环境中稳定运行，为用户提供可靠的代理服务。