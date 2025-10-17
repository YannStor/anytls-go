# AnyTLS-Go 项目概览

## 🎯 项目简介

AnyTLS-Go 是一个高性能的 TLS 代理服务器实现，旨在缓解嵌套 TLS 握指纹问题。本项目在原有基础上增加了强大的企业级代理功能，支持多代理列表、智能故障转移、健康检查等高级特性。

## ✨ 核心特性

### 🔄 代理功能
- **代理列表支持**: 支持多个代理服务器，自动故障转移
- **DIRECT 直连**: 将直连作为特殊代理类型统一管理
- **多协议支持**: SOCKS5、HTTP、HTTPS 代理协议
- **智能回切**: 前面代理恢复时优先回切到更靠前的代理

### 🏥 健康检查
- **自定义URL**: 支持自定义健康检查URL列表
- **可配置参数**: 检查间隔、超时时间、成功阈值
- **数据传输感知**: 有数据传输成功时跳过不必要的健康检查
- **多URL验证**: 使用多个检测URL提高可靠性

### 🌐 网络支持
- **TCP/UDP**: 完整支持 TCP 和 UDP 连接
- **IPv4/IPv6**: 双栈网络协议支持
- **SOCKS5 UDP**: 通过 UDP ASSOCIATE 实现 UDP 代理
- **多平台**: Linux、Windows、macOS、FreeBSD

### 📊 监控和管理
- **连接统计**: 实时监控活跃连接数
- **版本信息**: 构建时注入版本、时间、提交信息
- **优雅关闭**: 支持 SIGINT/SIGTERM 信号处理
- **详细日志**: 结构化日志记录

## 📁 项目结构

```
anytls-go/
├── cmd/                    # 主程序入口
│   ├── server/            # 服务器端
│   └── client/            # 客户端
├── proxy/                 # 代理功能模块
│   ├── dialer/           # 代理拨号器（新增）
│   ├── padding/          # 填充策略
│   ├── session/          # 会话管理
│   ├── pipe/             # 管道处理
│   └── system_dialer.go  # 系统拨号器
├── util/                  # 工具函数
├── examples/              # 配置示例（新增）
├── docs/                  # 文档目录
├── .github/workflows/     # GitHub Actions（新增）
└── *.md                   # 项目文档
```

## 🚀 快速开始

### 基础使用
```bash
# 启动服务器（无代理）
./anytls-server -l 0.0.0.0:8443 -p password

# 启动客户端
./anytls-client -l 127.0.0.1:1080 -s server_ip:8443 -p password
```

### 代理配置
```bash
# 单个代理
./anytls-server -l 0.0.0.0:8443 -p password \
  -dial "socks5://user:pass@127.0.0.1:1080"

# 代理列表（高可用）
./anytls-server -l 0.0.0.0:8443 -p password \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT" \
  -health-urls "https://cp.cloudflare.com/,https://www.google.com" \
  -health-interval 1m -health-threshold 2
```

## 🔧 配置参数

### 基础参数
- `-l`: 监听地址和端口
- `-p`: 连接密码
- `-n`: TLS 服务器名称

### 代理参数
- `-dial`: 代理URL列表（逗号分隔）
- `-dialfallback`: 启用直连回退（等价于添加DIRECT）

### 健康检查参数
- `-health-urls`: 健康检查URL列表
- `-health-interval`: 检查间隔（默认30秒）
- `-health-timeout`: 超时时间（默认10秒）
- `-health-threshold`: 成功阈值（默认1）
- `-data-idle`: 数据传输空闲时间（默认5分钟）

### 连接参数
- `-connect-timeout`: 连接超时（默认30秒）
- `-read-timeout`: 读取超时（默认60秒）
- `-write-timeout`: 写入超时（默认60秒）

## 🧪 测试

```bash
# 运行单元测试
go test ./proxy/simpledialer -v

# 构建测试
./build.sh

# 清理项目
./clean.sh
```

## 📦 构建和部署

### 本地构建
```bash
# 构建所有平台
./build.sh

# 或简化构建
./build-simple.sh
```

### Docker 构建
```bash
# 构建镜像
docker build -t anytls-go .

# 运行容器
docker run -p 8443:8443 -p 1080:1080 anytls-go
```

## 🔄 CI/CD

项目配置了完整的 GitHub Actions 工作流：
- **自动测试**: 每个 PR 和 push 触发
- **自动构建**: 多平台交叉编译
- **自动发布**: 标签推送时创建 GitHub Release
- **Docker 镜像**: 自动构建和推送到 Docker Hub

## 📚 文档

- [USAGE.md](USAGE.md) - 详细使用说明
- [PROXY_FEATURE.md](PROXY_FEATURE.md) - 代理功能文档
- [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md) - 技术实现总结
- [examples/](examples/) - 配置示例
- [CHECKLIST.md](CHECKLIST.md) - 功能检查清单

## 🛡️ 安全特性

- **TLS 加密**: 完整的 TLS 连接支持
- **代理认证**: 支持 SOCKS5 和 HTTP 代理认证
- **密码保护**: 强制设置连接密码
- **错误处理**: 安全的错误信息处理，避免信息泄露

## 📈 性能特性

- **智能跳过**: 数据传输成功时跳过健康检查
- **连接复用**: 优化的连接管理
- **故障快速检测**: 即时响应传输失败
- **并发安全**: 线程安全的代理切换

## 🎯 适用场景

- **网络代理**: 企业内部网络代理
- **负载均衡**: 多服务器负载分配
- **故障转移**: 高可用代理服务
- **网络安全**: TLS 隧道和流量加密

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用开源许可证，具体请查看 LICENSE 文件。

## 📞 支持

如有问题，请：
1. 查看 [USAGE.md](USAGE.md)
2. 搜索现有 Issues
3. 创建新的 Issue

---

**版本**: v0.0.11  
**最后更新**: 2025-10-17  
**维护者**: YannStor