# AnyTLS-Go 使用说明

## 📋 目录

- [快速开始](#快速开始)
- [代理功能](#代理功能)
- [配置示例](#配置示例)
- [故障排除](#故障排除)
- [性能优化](#性能优化)

## 🚀 快速开始

### 基础使用

```bash
# 启动服务器
./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p yourpassword

# 启动客户端
./anytls-client-linux-amd64 -l 127.0.0.1:1080 -s server_ip:8443 -p yourpassword
```

### 验证连接

```bash
# 测试 SOCKS5 代理
curl --socks5 127.0.0.1:1080 http://httpbin.org/ip

# 测试 HTTP 代理
curl -x http://127.0.0.1:1080 http://httpbin.org/ip
```

## 🔧 代理功能

### 代理列表配置

支持多个代理服务器，自动故障转移：

```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p password \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,socks5://proxy3.com:1080,DIRECT"
```

### DIRECT 直连

`DIRECT` 是特殊的代理类型，表示直接连接：

```bash
# 等价配置
./anytls-server-linux-amd64 -dial "socks5://proxy.com:1080,DIRECT"
./anytls-server-linux-amd64 -dial "socks5://proxy.com:1080" -dialfallback
```

### 健康检查配置

```bash
./anytls-server-linux-amd64 \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,DIRECT" \
  -health-urls "https://cp.cloudflare.com/,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 15s \
  -health-threshold 2 \
  -data-idle 10m
```

## 📝 配置示例

### 1. 单个 SOCKS5 代理

```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p password \
  -dial "socks5://username:password@127.0.0.1:1080"
```

### 2. 高可用代理配置

```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p securepassword \
  -n example.com \
  -dial "socks5://proxy1.com:1080,http://proxy2.com:8080,socks5://proxy3.com:1080,DIRECT" \
  -health-urls "https://cp.cloudflare.com/,https://connectivitycheck.gstatic.com/generate_204" \
  -health-interval 30s \
  -health-threshold 1
```

### 3. 国内网络优化配置

```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p password \
  -dial "socks5://proxy.com:1080,DIRECT" \
  -health-urls "http://wifi.vivo.com.cn/generate_204,https://cp.cloudflare.com/" \
  -health-interval 2m \
  -health-timeout 10s
```

### 4. 企业级配置

```bash
./anytls-server-linux-amd64 \
  -l 0.0.0.0:8443 \
  -p enterprise_password \
  -n enterprise.example.com \
  -dial "socks5://proxy1.internal.com:1080,socks5://proxy2.internal.com:1080,http://proxy3.internal.com:8080,DIRECT" \
  -health-urls "https://monitoring.internal.com/health,https://www.google.com" \
  -health-interval 1m \
  -health-timeout 10s \
  -health-threshold 2 \
  -data-idle 5m
```

## 🔍 故障排除

### 常见问题

#### 1. 代理连接失败

**症状**: 代理无法连接，日志显示连接错误

**解决方案**:
```bash
# 检查代理服务器状态
curl --socks5 username:password@proxy.com:1080 http://httpbin.org/ip

# 使用 telnet 测试连接
telnet proxy.com 1080

# 检查防火墙设置
sudo ufw status
```

#### 2. 健康检查失败

**症状**: 代理频繁切换，健康检查失败

**解决方案**:
```bash
# 使用更可靠的健康检查URL
-health-urls "https://8.8.8.8,https://1.1.1.1"

# 增加超时时间
-health-timeout 30s

# 降低成功阈值
-health-threshold 1
```

#### 3. 性能问题

**症状**: 连接速度慢，延迟高

**解决方案**:
```bash
# 减少健康检查频率
-health-interval 5m

# 增加数据传输空闲时间
-data-idle 15m

# 使用地理位置更近的代理
-dial "socks5://nearest-proxy.com:1080,DIRECT"
```

### 日志分析

```bash
# 启用详细日志
LOG_LEVEL=debug ./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password

# 关键日志信息
[Server] Using outbound proxy: socks5://user:***@proxy.com:1080
[Server] Proxy fallback enabled
[Server] Health check: 2 URLs, interval: 30s, timeout: 10s, threshold: 1
[Server] Proxy marked as unhealthy after 3 consecutive failures
[Server] Proxy health check passed, resuming proxy usage
```

## ⚡ 性能优化

### 1. 网络优化

```bash
# 使用更快的代理服务器
-dial "socks5://fast-proxy.com:1080,socks5://backup-proxy.com:1080,DIRECT"

# 优化健康检查
-health-urls "https://fast-server.com/health" \
-health-interval 2m \
-health-timeout 5s
```

### 2. 系统优化

```bash
# 增加文件描述符限制
ulimit -n 65535

# 调整内核参数
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 65535' >> /etc/sysctl.conf
sysctl -p
```

### 3. 内存优化

```bash
# 减少 Go 程序内存占用
GOGC=100 ./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password

# 使用更少的健康检查URL
-health-urls "https://single-reliable-url.com"
```

## 📊 监控和维护

### 状态检查

```bash
# 检查进程状态
ps aux | grep anytls-server

# 检查端口监听
netstat -tlnp | grep :8443

# 检查连接数
ss -an | grep :8443 | wc -l
```

### 自动重启脚本

```bash
#!/bin/bash
# restart.sh
while true; do
    ./anytls-server-linux-amd64 -l 0.0.0.0:8443 -p password \
        -dial "socks5://proxy.com:1080,DIRECT"
    echo "服务器重启中..."
    sleep 5
done
```

### systemd 服务配置

```ini
# /etc/systemd/system/anytls.service
[Unit]
Description=AnyTLS Server
After=network.target

[Service]
Type=simple
User=anytls
ExecStart=/opt/anytls/anytls-server-linux-amd64 \
    -l 0.0.0.0:8443 \
    -p password \
    -dial "socks5://proxy.com:1080,DIRECT"
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# 启用服务
sudo systemctl enable anytls
sudo systemctl start anytls
sudo systemctl status anytls
```

## 🔒 安全建议

### 1. 密码安全

```bash
# 使用强密码
-p "$(openssl rand -base64 32)"

# 避免在命令行中暴露密码
echo "password" > /opt/anytls/password.txt
chmod 600 /opt/anytls/password.txt
-p "$(cat /opt/anytls/password.txt)"
```

### 2. 代理认证

```bash
# 使用加密的代理连接
-dial "socks5://user:complex_password@secure-proxy.com:1080"

# 定期更换代理密码
```

### 3. 网络安全

```bash
# 限制访问IP
-l 127.0.0.1:8443  # 仅本地访问

# 使用防火墙限制访问
sudo ufw allow from trusted_ip to any port 8443
```

## 📞 支持

如果遇到问题，请：

1. 查看日志文件
2. 检查网络连接
3. 验证代理配置
4. 参考本文档的故障排除部分

---

**版本**: v0.0.11  
**更新时间**: 2025-10-17  
**支持的代理协议**: SOCKS5, HTTP, HTTPS, DIRECT  
**支持的平台**: Linux, Windows, macOS, FreeBSD