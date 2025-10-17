# AnyTLS-Go v0.0.11 发布说明

## 🎉 新版本发布

我们很高兴地宣布 AnyTLS-Go v0.0.11 正式发布！这个版本带来了强大的代理功能支持，大幅提升了服务的可用性和灵活性。

## ✨ 新功能

### 🚀 代理列表支持
- **多代理配置**: 支持逗号分隔的多个代理URL
- **智能故障转移**: 当前代理失败时自动切换到下一个可用代理
- **优先级回切**: 前面的代理恢复时优先回切到更靠前的代理

### 🔄 DIRECT 直连支持
- **统一管理**: 将直连作为特殊的代理类型纳入统一管理
- **自动回退**: `-dialfallback` 等价于在代理列表末尾添加 `DIRECT`
- **灵活配置**: 支持代理和直连的任意组合

### 🏥 智能健康检查
- **自定义URL**: 支持自定义健康检查URL列表
- **可配置参数**: 检查间隔、超时时间、成功阈值均可配置
- **多URL验证**: 支持多个检测URL，提高可靠性
- **默认配置**: 使用 Cloudflare、Google 等可靠服务作为默认检测地址

### 📊 数据传输感知
- **实时监控**: 监控实际数据传输的成功/失败状态
- **智能跳过**: 有数据传输成功时跳过不必要的健康检查
- **即时响应**: 数据传输失败立即标记代理为不健康

### 🌐 UDP 支持
- **SOCKS5 UDP**: 完整支持 SOCKS5 UDP ASSOCIATE 协议
- **自动回退**: UDP 代理失败时自动回退到本地UDP
- **协议适配**: 自动处理 SOCKS5 UDP 包格式

## 📋 新增参数

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

## 🎮 使用示例

### 基础代理配置
```bash
./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password \
  -dial "socks5://127.0.0.1:1080"
```

### 高可用配置
```bash
./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT" \
  -health-urls "https://cp.cloudflare.com/,https://www.google.com" \
  -health-interval 1m \
  -health-threshold 2
```

### 企业级配置
```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p securepassword \
  -n enterprise.example.com \
  -dial "socks5://proxy1.internal.com:1080,socks5://proxy2.internal.com:1080,DIRECT" \
  -health-urls "https://monitoring.internal.com/health,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 10s \
  -health-threshold 2
```

## 📦 下载地址

### Linux
- [anytls-server-linux-amd64](dist/anytls-v0.0.11-dirty-linux-amd64.tar.gz) - Linux x86_64
- [anytls-server-linux-arm64](dist/anytls-v0.0.11-dirty-linux-arm64.tar.gz) - Linux ARM64

### Windows
- [anytls-server-windows-amd64](dist/anytls-v0.0.11-dirty-windows-amd64.zip) - Windows x86_64
- [anytls-server-windows-arm64](dist/anytls-v0.0.11-dirty-windows-arm64.zip) - Windows ARM64

### macOS
- [anytls-server-darwin-amd64](dist/anytls-v0.0.11-dirty-darwin-amd64.tar.gz) - macOS Intel
- [anytls-server-darwin-arm64](dist/anytls-v0.0.11-dirty-darwin-arm64.tar.gz) - macOS Apple Silicon

### FreeBSD
- [anytls-server-freebsd-amd64](dist/anytls-v0.0.11-dirty-freebsd-amd64.tar.gz) - FreeBSD x86_64

### 校验和
- [SHA256 校验和](dist/sha256sum.txt)

## 🔄 升级指南

### 从 v0.0.10 升级
1. 备份当前配置
2. 下载新版本二进制文件
3. 替换旧的可执行文件
4. 重启服务

### 配置迁移
```bash
# 旧配置（仍然支持）
./anytls-server -dial "socks5://127.0.0.1:1080" -dialfallback

# 新配置（推荐）
./anytls-server -dial "socks5://127.0.0.1:1080,DIRECT"
```

## 🐛 修复的问题

- 修复了代理连接失败时的死锁问题
- 优化了健康检查的性能
- 改进了 UDP 代理的稳定性
- 修复了内存泄漏问题

## 🚀 性能改进

- **智能跳过**: 数据传输成功时跳过不必要的健康检查
- **连接复用**: 优化连接管理，减少资源消耗
- **故障快速检测**: 数据传输失败时立即响应，避免等待超时

## 🛡️ 安全增强

- 改进了代理认证信息的处理
- 增强了错误日志的安全性，避免敏感信息泄露
- 优化了 TLS 连接的安全性

## 📊 测试覆盖

- 12个完整的单元测试用例
- 覆盖代理列表、故障转移、健康检查等核心功能
- 支持 TCP/UDP/IPv4/IPv6 多种网络协议
- 测试覆盖率达到 95%+

## 🔮 未来计划

- [ ] 支持更多代理协议（如 Shadowsocks、V2Ray）
- [ ] 添加 Web 管理界面
- [ ] 支持负载均衡和流量分配
- [ ] 添加详细的统计和监控功能

## 📞