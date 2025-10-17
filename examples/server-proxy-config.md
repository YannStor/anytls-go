# AnyTLS-Go 服务器代理配置示例

## 基本用法

### 1. 使用 SOCKS5 代理（带认证）- 支持 TCP 和 UDP
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "socks5://username:password@127.0.0.1:1080"
```

### 2. 使用 SOCKS5 代理（无认证）- 支持 TCP 和 UDP
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "socks5://127.0.0.1:1080"
```

### 3. 使用 HTTP 代理（带认证）- 仅支持 TCP
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "http://username:password@127.0.0.1:8080"
```

### 4. 使用 HTTP 代理（无认证）- 仅支持 TCP
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "http://127.0.0.1:8080"
```

### 5. 使用 HTTPS 代理 - 仅支持 TCP
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "https://username:password@proxy.example.com:443"
```

## 高级配置

### 6. 启用动态代理失败回退
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword -dial "socks5://127.0.0.1:1080" -dialfallback
```

当代理连接失败时，服务器会自动回退到直连模式。支持动态回退：
- 代理连续失败 3 次后自动标记为不健康
- 不健康期间使用直连
- 自动检查代理恢复状态（可配置间隔）
- 代理恢复后自动重新使用

### 7. 自定义健康检查配置
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword \
  -dial "socks5://127.0.0.1:1080" \
  -dialfallback \
  -health-urls "https://httpbin.org/status/200,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 15s \
  -health-threshold 2
```

**健康检查参数说明**：
- `-health-urls`: 自定义健康检查 URL 列表（逗号分隔）
- `-health-interval`: 健康检查间隔（默认 30 秒）
- `-health-timeout`: 单次健康检查超时时间（默认 10 秒）
- `-health-threshold`: 需要成功检查的数量阈值（默认 1）

### 8. 使用默认健康检查配置
```bash
./anytls-server -l 0.0.0.0:8443 -p yourpassword \
  -dial "socks5://127.0.0.1:1080" \
  -dialfallback
```

默认使用以下URL进行健康检查：
- https://cp.cloudflare.com/
- https://connectivitycheck.gstatic.com/generate_204
- http://wifi.vivo.com.cn/generate_204
- http://www.google.com/generate_204

### 9. 完整配置示例
```bash
./anytls-server \
  -l 0.0.0.0:8443 \
  -p yourSecurePassword \
  -n example.com \
  -dial "socks5://user:pass@proxy.example.com:1080" \
  -dialfallback \
  -health-urls "https://cp.cloudflare.com/,https://connectivitycheck.gstatic.com/generate_204,http://wifi.vivo.com.cn/generate_204,http://www.google.com/generate_204" \
  -health-interval 30s \
  -health-timeout 10s \
  -health-threshold 1 \
  -padding-scheme /path/to/padding-scheme.json
```

## 支持的代理协议

- **SOCKS5**: `socks5://[username:password@]host:port` - 支持 TCP 和 UDP
- **HTTP**: `http://[username:password@]host:port` - 仅支持 TCP
- **HTTPS**: `https://[username:password@]host:port` - 仅支持 TCP

## 认证支持

代理配置支持完整的用户名/密码认证，认证信息会自动编码为适当的格式：

- SOCKS5 代理使用标准用户名/密码认证
- HTTP/HTTPS 代理使用 Basic Authentication

## UDP 支持

### SOCKS5 UDP 代理
- SOCKS5 代理完全支持 UDP 连接
- 通过 UDP ASSOCIATE 方法实现
- 自动处理 SOCKS5 UDP 包格式
- 适用于 DNS 查询等 UDP 流量

### UDP 回退机制
- 当 SOCKS5 UDP 代理失败时，会回退到本地 UDP
- 确保 UDP 流量的可靠性

## 动态回退机制

### 健康检查
- 自动通过HTTP/HTTPS请求监测代理连接状态
- 连续失败3次后标记代理为不健康
- 按配置的间隔自动尝试恢复代理连接
- 支持多个检测URL，只需达到成功阈值即认为健康

### 回退策略
- 代理健康时：优先使用代理
- 代理不健康时：自动使用直连
- 代理恢复时：自动切换回代理
- 检测失败和恢复都会在日志中记录

## 注意事项

1. 代理 URL 必须使用有效的 scheme（socks5、http、https）
2. 用户名和密码如果包含特殊字符，需要进行 URL 编码
3. SOCKS5 代理支持 TCP 和 UDP，HTTP/HTTPS 代理仅支持 TCP
4. 动态回退机制确保服务的高可用性
5. 代理健康状态会在日志中显示
6. 健康检查URL应该是响应快速且可靠的服务
7. 检查间隔不宜过短，避免对检测服务器造成压力

## 测试代理连接

可以使用以下命令测试代理是否正常工作：

```bash
# 测试 SOCKS5 TCP 代理
curl --socks5 username:password@127.0.0.1:1080 http://httpbin.org/ip

# 测试 SOCKS5 UDP 代理（DNS查询）
dig @8.8.8.8 google.com

# 测试 HTTP 代理
curl -x http://username:password@127.0.0.1:8080 http://httpbin.org/ip

# 测试健康检查URL
curl -I https://cp.cloudflare.com/
curl -I https://connectivitycheck.gstatic.com/generate_204
curl -I http://wifi.vivo.com.cn/generate_204
curl -I http://www.google.com/generate_204
```

## 日志监控

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

### 代理连接失败
1. 检查代理服务器是否正常运行
2. 验证代理认证信息是否正确
3. 确认网络连接可达代理服务器
4. 查看服务器日志了解具体错误

### UDP 代理问题
1. 确保使用 SOCKS5 代理（HTTP 代理不支持 UDP）
2. 检查代理服务器是否启用 UDP ASSOCIATE
3. 验证防火墙设置允许 UDP 流量

### 健康检查问题
1. 确认健康检查URL是否可访问
2. 检查网络连接是否正常
3. 调整健康检查超时时间
4. 增加成功阈值提高可靠性
5. 使用更可靠的健康检查URL

### 动态回退不工作
1. 确认启用了 `-dialfallback` 参数
2. 检查代理是否真的无法连接
3. 查看日志中的健康状态变化
4. 验证健康检查URL是否可访问
5. 调整检查间隔和超时时间

### 健康检查配置建议
- **国内环境**: 使用国内URL如 `http://wifi.vivo.com.cn/generate_204`
- **国际环境**: 使用 `https://cp.cloudflare.com/` 或 `https://www.google.com`
- **高可靠性**: 设置阈值为 2-3，需要多个 URL 成功
- **快速检测**: 减少超时时间到 5 秒，间隔设为 1 分钟
- **低频检测**: 增加间隔到 5-10 分钟，减少资源消耗
