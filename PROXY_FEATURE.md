# AnyTLS-Go 出站代理功能

## 功能概述

为 anytls-go 服务器端添加了完整的出站代理支持，允许服务器通过指定的代理服务器转发所有出站连接。支持 TCP 和 UDP 流量，具备动态回退机制确保高可用性。

## 新增参数

### `-dial`
- **描述**: 出站代理 URL
- **格式**: 
  - SOCKS5: `socks5://[username:password@]host:port` - 支持 TCP 和 UDP
  - HTTP: `http://[username:password@]host:port` - 仅支持 TCP
  - HTTPS: `https://[username:password@]host:port` - 仅支持 TCP
- **必需**: 否
- **示例**: 
  ```bash
  -dial "socks5://user:pass@127.0.0.1:1080"
  -dial "http://user:pass@127.0.0.1:8080"
  ```

### `-dialfallback`
- **描述**: 启用动态代理失败回退机制
- **默认值**: `false`
- **类型**: 布尔值
- **功能**: 
  - 代理连续失败3次后自动标记为不健康
  - 不健康期间使用直连替代
  - 自动检查代理恢复状态（可配置间隔）
  - 代理恢复后自动重新使用
- **示例**: 
  ```bash
  -dialfallback
  ```

### `-health-urls`
- **描述**: 健康检查URL列表，逗号分隔
- **默认值**: `https://cp.cloudflare.com/,https://connectivitycheck.gstatic.com/generate_204,http://wifi.vivo.com.cn/generate_204,http://www.google.com/generate_204`
- **类型**: 字符串
- **示例**: 
  ```bash
  -health-urls "https://httpbin.org/status/200,https://www.google.com"
  ```

### `-health-interval`
- **描述**: 健康检查间隔
- **默认值**: `30s`
- **类型**: 时间段
- **示例**: 
  ```bash
  -health-interval 1m
  ```

### `-health-timeout`
- **描述**: 单次健康检查超时时间
- **默认值**: `10s`
- **类型**: 时间段
- **示例**: 
  ```bash
  -health-timeout 15s
  ```

### `-health-threshold`
- **描述**: 需要成功检查的数量阈值
- **默认值**: `1`
- **类型**: 整数
- **示例**: 
  ```bash
  -health-threshold 2
  ```

## 支持的代理协议

### SOCKS5 代理
- 完整支持 SOCKS5 协议
- 支持用户名/密码认证
- 支持 TCP 和 UDP 连接
- 通过 UDP ASSOCIATE 实现 UDP 代理
- 自动处理 SOCKS5 UDP 包格式

### HTTP/HTTPS 代理
- 支持 HTTP CONNECT 方法
- 支持 Basic Authentication
- 完整的头部处理
- 仅支持 TCP 连接

### UDP 支持特性
- SOCKS5 代理完全支持 UDP 流量
- 自动处理 UDP ASSOCIATE 协议
- UDP 代理失败时回退到本地 UDP
- 适用于 DNS 查询等 UDP 应用

## 认证支持

代理配置支持完整的用户名/密码认证：

1. **SOCKS5 认证**: 使用标准的用户名/密码认证方法
2. **HTTP 认证**: 使用 HTTP Basic Authentication
3. **URL 编码**: 特殊字符会自动进行 URL 编码处理

## 实现细节

### 核心组件

1. **SimpleDialer** (`proxy/simpledialer/dialer.go`)
   - 简化的代理拨号器接口
   - 支持多种代理协议
   - 实现故障转移和回切机制

2. **HTTPProxyDialer** 
   - HTTP/HTTPS 代理的具体实现
   - 处理 CONNECT 请求和响应

3. **myServer 增强**
   - 集成代理配置
   - 动态拨号器选择

### 工作流程

1. 服务器启动时解析代理配置并初始化代理拨号器
2. 配置健康检查参数（URL列表、间隔、超时、阈值）
3. 出站连接时检查代理健康状态
4. 代理健康时优先使用代理拨号器
5. 记录连接成功/失败，更新健康状态
6. 代理连续失败3次后标记为不健康
7. 不健康期间使用直连（如果启用回退）
8. 按配置间隔定期检查代理恢复状态
9. 通过HTTP/HTTPS请求验证代理可用性
10. 达到成功阈值后标记代理为健康，自动切换回代理模式

### 动态回退机制
- **健康监测**: 通过HTTP/HTTPS请求实时监控代理连接状态
- **智能切换**: 自动在代理和直连间切换
- **故障恢复**: 自动检测并恢复使用代理
- **高可用**: 确保服务连续性
- **可配置**: 支持自定义检测URL、间隔、超时和阈值

## 使用示例

### 基本 SOCKS5 代理
### 带认证的 SOCKS5 代理（支持TCP+UDP）
```bash
./anytls-server -l 0.0.0.0:8443 -p password -dial "socks5://user:pass@127.0.0.1:1080"
```

### HTTP 代理（带认证，仅TCP）
```bash
./anytls-server -l 0.0.0.0:8443 -p password -dial "http://user:pass@proxy.com:8080"
```

### 启用动态回退机制（使用默认健康检查）
```bash
./anytls-server -l 0.0.0.0:8443 -p password -dial "socks5://127.0.0.1:1080" -dialfallback
```

### 自定义健康检查配置
```bash
./anytls-server -l 0.0.0.0:8443 -p password \
  -dial "socks5://127.0.0.1:1080" \
  -dialfallback \
  -health-urls "https://httpbin.org/status/200,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 15s \
  -health-threshold 2
```

### 完整配置（高可用性）
```bash
./anytls-server \
  -l 0.0.0.0:8443 \
  -p password \
  -n example.com \
  -dial "socks5://user:pass@proxy.example.com:1080" \
  -dialfallback \
  -health-urls "https://cp.cloudflare.com/,https://connectivitycheck.gstatic.com/generate_204,http://wifi.vivo.com.cn/generate_204,http://www.google.com/generate_204" \
  -health-interval 30s \
  -health-timeout 10s \
  -health-threshold 1 \
  -padding-scheme /path/to/padding-scheme.json
```

## 兼容性

- **向后兼容**: 不指定 `-dial` 参数时，行为与原版本完全一致
- **网络支持**: 
  - SOCKS5 代理支持 TCP 和 UDP
  - HTTP/HTTPS 代理支持 TCP
  - UDP over TCP 支持代理和回退
- **错误处理**: 完善的错误处理和日志记录
- **高可用**: 动态回退确保服务连续性

## 测试覆盖

包含完整的单元测试：
- 代理拨号器创建测试（8个用例）
- 多协议支持测试（TCP/UDP/IPv4/IPv6）
- 认证功能测试
- 动态回退机制测试
- 健康状态恢复测试
- 健康检查配置测试
- URL健康检查测试
- 自定义健康检查测试
- 错误处理测试

运行测试：
```bash
go test ./proxy/simpledialer -v
```

## 安全注意事项

1. **代理认证**: 确保代理认证信息安全
2. **URL 编码**: 特殊字符需要进行正确的 URL 编码
3. **回退机制**: 谨慎使用回退功能，避免意外暴露真实 IP

## 依赖更新

新增依赖：
- `golang.org/x/net/proxy`: 用于 SOCKS5 代理支持

## 构建验证

```bash
# 构建服务器
go build -o bin/anytls-server ./cmd/server

# 构建客户端（保持兼容）
go build -o bin/anytls-client ./cmd/client

# 运行测试
go test ./...
```

## 日志输出

启用代理时的日志示例：
```
[Server] Using outbound proxy: socks5://user:***@127.0.0.1:1080
[Server] Proxy fallback enabled
[Server] Health check: 4 URLs, interval: 30s, timeout: 10s, threshold: 1
```

运行时的健康状态日志：
```
[Server] Proxy marked as unhealthy after 3 consecutive failures
[Server] Proxy health check passed, resuming proxy usage
```

无代理时的日志：
```
[Server] Using direct outbound connection
```

## 故障排除

### 代理连接问题
1. 检查代理服务器状态和认证信息
2. 验证网络连通性
3. 查看服务器日志了解具体错误
4. 确认代理协议（SOCKS5支持UDP，HTTP仅支持TCP）

### 健康检查问题
1. 确认健康检查URL是否可访问
2. 检查网络连接是否正常
3. 调整健康检查超时时间
4. 增加成功阈值提高可靠性
5. 使用更可靠的健康检查URL

### 动态回退问题
1. 确认启用了 `-dialfallback` 参数
2. 检查代理是否真的无法连接
3. 查看日志中的健康状态变化
4. 验证健康检查URL是否可访问
5. 调整检查间隔和超时时间

### 健康检查配置建议
- **国内环境**: 使用国内URL如 `http://wifi.vivo.com.cn/generate_204`
- **国际环境**: 使用 `https://cp.cloudflare.com/` 或 `https://www.google.com`
- **高可靠性**: 设置阈值为2-3，需要多个URL成功
- **快速检测**: 减少超时时间到5秒，间隔设为1分钟
- **低频检测**: 增加间隔到5-10分钟，减少资源消耗

### UDP 代理问题
1. 必须使用 SOCKS5 代理
2. 检查代理服务器是否启用 UDP ASSOCIATE
3. 验证防火墙允许 UDP 流量
4. UDP 代理失败会自动回退到本地 UDP
