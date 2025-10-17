package dialer

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"
)

func TestNewProxyDialer(t *testing.T) {
	tests := []struct {
		name        string
		dialURL     string
		fallback    bool
		expectError bool
	}{
		{
			name:        "Valid SOCKS5 proxy without auth",
			dialURL:     "socks5://127.0.0.1:1080",
			fallback:    false,
			expectError: false,
		},
		{
			name:        "Valid SOCKS5 proxy with auth",
			dialURL:     "socks5://user:pass@127.0.0.1:1080",
			fallback:    false,
			expectError: false,
		},
		{
			name:        "Valid HTTP proxy without auth",
			dialURL:     "http://127.0.0.1:8080",
			fallback:    false,
			expectError: false,
		},
		{
			name:        "Valid HTTP proxy with auth",
			dialURL:     "http://user:pass@127.0.0.1:8080",
			fallback:    false,
			expectError: false,
		},
		{
			name:        "Valid HTTPS proxy",
			dialURL:     "https://user:pass@proxy.example.com:443",
			fallback:    false,
			expectError: false,
		},
		{
			name:        "Invalid scheme",
			dialURL:     "ftp://127.0.0.1:21",
			fallback:    false,
			expectError: true,
		},
		{
			name:        "Empty URL",
			dialURL:     "",
			fallback:    false,
			expectError: true,
		},
		{
			name:        "Invalid URL format",
			dialURL:     "not-a-url",
			fallback:    false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer, err := NewProxyDialer(tt.dialURL, tt.fallback)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if dialer == nil {
				t.Errorf("Expected dialer but got nil")
				return
			}

			// 验证 fallback 设置
			if dialer.fallback != tt.fallback {
				t.Errorf("Expected fallback %v, got %v", tt.fallback, dialer.fallback)
			}
		})
	}
}

func TestProxyDialerDialContext(t *testing.T) {
	// 这个测试需要实际的代理服务器才能运行
	// 在没有代理服务器的情况下，我们只测试接口实现

	t.Run("SOCKS5 proxy interface", func(t *testing.T) {
		dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
		if err != nil {
			t.Skipf("Cannot create SOCKS5 dialer: %v", err)
		}

		// 测试 DialContext 方法存在
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		// 这里会因为代理服务器不存在而失败，但我们只是验证方法调用不会 panic
		conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Logf("Expected connection failure (no proxy server): %v", err)
		}
		if conn != nil {
			conn.Close()
		}
	})

	t.Run("HTTP proxy interface", func(t *testing.T) {
		dialer, err := NewProxyDialer("http://127.0.0.1:8080", true)
		if err != nil {
			t.Skipf("Cannot create HTTP dialer: %v", err)
		}

		// 测试 DialContext 方法存在
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Logf("Expected connection failure (no proxy server): %v", err)
		}
		if conn != nil {
			conn.Close()
		}
	})
}

func TestHTTPProxyDialer(t *testing.T) {
	proxyURL, err := url.Parse("http://user:pass@127.0.0.1:8080")
	if err != nil {
		t.Fatalf("Failed to parse proxy URL: %v", err)
	}

	dialer := &HTTPProxyDialer{
		proxyURL: proxyURL,
		dialer: net.Dialer{
			Timeout: time.Second * 5,
		},
	}

	// 测试 Dial 方法存在
	conn, err := dialer.Dial("tcp", "example.com:80")
	if err != nil {
		t.Logf("Expected connection failure (no proxy server): %v", err)
	}
	if conn != nil {
		conn.Close()
	}

	// 测试 DialContext 方法存在
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	conn, err = dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Expected connection failure (no proxy server): %v", err)
	}
	if conn != nil {
		conn.Close()
	}
}

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		username string
		password string
		expected string
	}{
		{"user", "pass", "dXNlcjpwYXNz"},
		{"test@example.com", "mypassword", "dGVzdEBleGFtcGxlLmNvbTpteXBhc3N3b3Jk"},
		{"", "", "Og=="},
		{"user", "", "dXNlcjo="},
		{"", "pass", "OnBhc3M="},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			result := basicAuth(tt.username, tt.password)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestProxyDialerFallback(t *testing.T) {
	// 测试回退功能
	dialer, err := NewProxyDialer("socks5://127.0.0.1:99999", true) // 使用不存在的端口
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	// 由于开启了回退，这应该尝试直连（如果目标可达）
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Connection failed (expected in many environments): %v", err)
	}
	if conn != nil {
		conn.Close()
		t.Log("Direct connection succeeded (fallback worked)")
	}
}

func TestSupportedNetworks(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	ctx := context.Background()

	// 测试支持的协议
	networks := []string{"tcp", "tcp4", "udp", "udp4"}
	for _, network := range networks {
		conn, err := dialer.DialContext(ctx, network, "example.com:53")
		if err != nil {
			t.Logf("Expected connection failure for %s (no proxy server): %v", network, err)
		} else {
			defer conn.Close()
		}
	}

	// 单独测试 tcp6 和 udp6（可能在某些环境中不可用）
	ipv6Networks := []string{"tcp6", "udp6"}
	for _, network := range ipv6Networks {
		conn, err := dialer.DialContext(ctx, network, "example.com:53")
		if err != nil {
			t.Logf("Expected connection failure for %s (IPv6 not available): %v", network, err)
		} else {
			defer conn.Close()
		}
	}

	// 测试不支持的协议
	_, err = dialer.DialContext(ctx, "unix", "/tmp/socket")
	if err == nil {
		t.Error("Expected error for UNIX network")
	}
}

func TestDynamicFallback(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:99999", true) // 使用不存在的端口
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	ctx := context.Background()

	// 初始状态应该是健康的
	health := dialer.GetProxyHealth()
	if !health.Healthy {
		t.Error("Initial proxy health should be healthy")
	}

	// 连续失败几次，应该触发回退
	for i := 0; i < 5; i++ {
		_, err := dialer.DialContext(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Logf("Expected connection failure %d: %v", i+1, err)
		}
	}

	// 检查健康状态
	health = dialer.GetProxyHealth()
	if health.Healthy {
		t.Error("Proxy should be unhealthy after consecutive failures")
	}
	if health.FailCount < 3 {
		t.Errorf("Expected at least 3 failures, got %d", health.FailCount)
	}

	// 后续连接应该使用直连（如果可能）
	_, err = dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Direct connection also failed (expected in some environments): %v", err)
	} else {
		t.Log("Direct connection succeeded (fallback working)")
	}
}

func TestProxyHealthRecovery(t *testing.T) {
	// 这个测试模拟代理恢复的情况
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 手动设置为不健康状态
	dialer.mu.Lock()
	if len(dialer.proxyNodes) > 0 {
		dialer.proxyNodes[0].Healthy = false
		dialer.proxyNodes[0].FailCount = 5
		dialer.proxyNodes[0].LastCheckTime = time.Now().Add(-time.Minute)
	}
	dialer.consecutiveFails = 5
	dialer.lastCheckTime = time.Now().Add(-time.Minute) // 设置为过去的时间
	dialer.mu.Unlock()

	// 检查初始状态
	health := dialer.GetProxyHealth()
	if health.Healthy {
		t.Error("Proxy should be unhealthy initially")
	}

	// 尝试连接，应该触发健康检查
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	_, err = dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Connection failed (expected if proxy not available): %v", err)
	}

	// 检查健康状态是否更新（由于连接失败，可能仍然不健康，但检查时间应该更新）
	health = dialer.GetProxyHealth()
	if time.Since(health.LastCheck) > time.Minute {
		t.Error("Health check should have been performed")
	}
}

func TestHealthCheckConfig(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 测试默认配置
	urls, interval, timeout, threshold := dialer.GetHealthCheckConfig()
	if len(urls) != 4 {
		t.Errorf("Expected 4 default URLs, got %d", len(urls))
	}
	if interval != time.Second*30 {
		t.Errorf("Expected 30s interval, got %v", interval)
	}
	if timeout != time.Second*10 {
		t.Errorf("Expected 10s timeout, got %v", timeout)
	}
	if threshold != 1 {
		t.Errorf("Expected threshold 1, got %d", threshold)
	}

	// 测试自定义配置
	customURLs := []string{"https://example.com", "http://test.com"}
	customInterval := time.Minute
	customTimeout := time.Second * 5
	customThreshold := 2

	dialer.SetHealthCheckConfig(customURLs, customInterval, customTimeout, customThreshold)

	urls, interval, timeout, threshold = dialer.GetHealthCheckConfig()
	if len(urls) != 2 {
		t.Errorf("Expected 2 custom URLs, got %d", len(urls))
	}
	if interval != customInterval {
		t.Errorf("Expected custom interval, got %v", interval)
	}
	if timeout != customTimeout {
		t.Errorf("Expected custom timeout, got %v", timeout)
	}
	if threshold != customThreshold {
		t.Errorf("Expected custom threshold, got %d", threshold)
	}
}

func TestCheckURLHealth(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 测试无效URL
	result := dialer.checkURLHealth("invalid-url")
	if result {
		t.Error("Invalid URL should return false")
	}

	// 测试不存在的主机（会失败，但验证方法存在）
	result = dialer.checkURLHealth("http://nonexistent.invalid.domain")
	if result {
		t.Error("Nonexistent domain should return false")
	}
}

func TestCustomHealthFallback(t *testing.T) {
	// 创建自定义健康检查配置
	dialer, err := NewProxyDialer("socks5://127.0.0.1:99999", true) // 不存在的代理
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 设置自定义URL和更快的检查间隔
	customURLs := []string{"https://httpbin.org/status/200"}
	dialer.SetHealthCheckConfig(customURLs, time.Second*2, time.Second*2, 1)

	ctx := context.Background()

	// 连续失败几次，应该触发回退
	for i := 0; i < 5; i++ {
		_, err := dialer.DialContext(ctx, "tcp", "example.com:80")
		if err != nil {
			t.Logf("Expected connection failure %d: %v", i+1, err)
		}
	}

	// 检查健康状态
	health := dialer.GetProxyHealth()
	if health.Healthy {
		t.Error("Proxy should be unhealthy after consecutive failures")
	}
}

func TestDataTransferAwareness(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 测试初始状态
	health := dialer.GetProxyHealth()
	// 代理列表中可能有多个代理，其中至少一个应该是健康的
	healthList := dialer.GetAllProxyHealth()
	healthyCount := 0
	for _, h := range healthList {
		if h.Healthy {
			healthyCount++
		}
	}
	if healthyCount == 0 {
		t.Error("At least one proxy should be healthy initially")
	}

	// 记录数据传输成功
	dialer.recordDataTransferSuccess()

	// 检查最近成功时间是否更新
	health = dialer.GetProxyHealth()
	if time.Since(health.LastCheck) > time.Second {
		t.Error("Last success time should be recent")
	}

	// 记录数据传输失败
	dialer.recordDataTransferFailure()

	// 检查代理是否标记为不健康
	health = dialer.GetProxyHealth()
	if health.Healthy {
		t.Error("Proxy should be unhealthy after data transfer failure")
	}
}

func TestMonitoredConn(t *testing.T) {
	// 创建一个模拟连接
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 包装连接
	monitoredConn := &MonitoredConn{Conn: client, dialer: dialer}

	// 测试写入数据
	testData := []byte("test data")
	n, err := monitoredConn.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write length mismatch: expected %d, got %d", len(testData), n)
	}

	// 读取数据
	buf := make([]byte, 1024)
	n, err = server.Read(buf)
	if err != nil {
		t.Errorf("Server read failed: %v", err)
	}

	// 从监控连接读取
	n, err = monitoredConn.Read(buf)
	if err != nil {
		t.Errorf("Monitored read failed: %v", err)
	}

	// 检查健康状态
	health := dialer.GetProxyHealth()
	if !health.Healthy {
		t.Error("Proxy should be healthy after successful data transfer")
	}
}

func TestAdvancedHealthCheckConfig(t *testing.T) {
	dialer, err := NewProxyDialer("socks5://127.0.0.1:1080", true)
	if err != nil {
		t.Skipf("Cannot create SOCKS5 dialer: %v", err)
	}

	// 测试高级配置
	customURLs := []string{"https://example.com"}
	customInterval := time.Minute
	customTimeout := time.Second * 5
	customThreshold := 2
	customDataIdle := time.Minute * 10

	dialer.SetHealthCheckConfigAdvanced(customURLs, customInterval, customTimeout, customThreshold, customDataIdle)

	// 获取配置
	urls, interval, timeout, threshold, dataIdle := dialer.GetHealthCheckConfigAdvanced()

	if len(urls) != 1 {
		t.Errorf("Expected 1 URL, got %d", len(urls))
	}
	if interval != customInterval {
		t.Errorf("Expected custom interval, got %v", interval)
	}
	if timeout != customTimeout {
		t.Errorf("Expected custom timeout, got %v", timeout)
	}
	if threshold != customThreshold {
		t.Errorf("Expected custom threshold, got %d", threshold)
	}
	if dataIdle != customDataIdle {
		t.Errorf("Expected custom data idle, got %v", dataIdle)
	}
}

func TestProxyList(t *testing.T) {
	// 测试代理列表
	proxyList := "socks5://127.0.0.1:1080,http://127.0.0.1:8080,socks5://user:pass@127.0.0.1:1081"
	dialer, err := NewProxyDialer(proxyList, true)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer with list: %v", err)
	}

	// 检查当前代理URL
	currentURL := dialer.GetCurrentProxyURL()
	if currentURL == "" {
		t.Error("Current proxy URL should not be empty")
	}

	// 获取所有代理健康状态
	allHealth := dialer.GetAllProxyHealth()
	// 由于启用了fallback，会自动添加DIRECT，所以应该是4个节点
	if len(allHealth) != 4 {
		t.Errorf("Expected 4 proxy nodes (3 proxies + DIRECT from fallback), got %d", len(allHealth))
	}

	// 测试代理切换
	initialURL := dialer.GetCurrentProxyURL()

	// 模拟当前代理失败
	currentNode := dialer.getCurrentNode()
	if currentNode != nil {
		dialer.recordNodeFailure(currentNode)
		dialer.switchToNextProxy()
	}

	// 检查是否切换到下一个代理
	newURL := dialer.GetCurrentProxyURL()
	if newURL == initialURL {
		t.Error("Proxy should have switched to next node")
	}
}

func TestIntelligentFailback(t *testing.T) {
	// 测试智能回切机制
	proxyList := "socks5://127.0.0.1:1080,socks5://127.0.0.1:1081,socks5://127.0.0.1:1082"
	dialer, err := NewProxyDialer(proxyList, false)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer: %v", err)
	}

	// 模拟第一个代理失败
	dialer.proxyNodes[0].Healthy = false
	dialer.proxyNodes[0].FailCount = 3

	// 当前应该切换到第二个或第三个代理
	node := dialer.findHealthyProxy()
	if node == nil || node.Index == 0 {
		t.Error("Should not use the failed first proxy")
	}

	// 模拟第一个代理恢复
	dialer.mu.Lock()
	dialer.proxyNodes[0].Healthy = true
	dialer.proxyNodes[0].FailCount = 0
	dialer.lastSuccessTime = time.Now().Add(-time.Hour) // 确保不会跳过健康检查
	dialer.mu.Unlock()

	// 直接调用 findHealthyProxy，应该选择第一个健康的代理
	node = dialer.findHealthyProxy()
	if node == nil {
		t.Error("Should find a healthy proxy")
	} else if node.Index != 0 {
		t.Errorf("Should select first proxy (index 0), got index %d", node.Index)
	}

	// 检查当前代理索引
	if dialer.currentNode != 0 {
		t.Errorf("Current node should be 0, got %d", dialer.currentNode)
	}
}

func TestProxyStatus(t *testing.T) {
	// 测试代理状态获取
	proxyList := "socks5://127.0.0.1:1080,http://127.0.0.1:8080"
	dialer, err := NewProxyDialer(proxyList, true)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer: %v", err)
	}

	status := dialer.GetProxyStatus()

	// 检查状态信息
	proxyListInterface, ok := status["proxy_list"].([]interface{})
	if !ok {
		t.Error("proxy_list should be a list")
	}

	var proxyInfoList []map[string]interface{}
	for _, item := range proxyListInterface {
		if proxyInfo, ok := item.(map[string]interface{}); ok {
			proxyInfoList = append(proxyInfoList, proxyInfo)
		}
	}
	if len(proxyInfoList) != 2 {
		t.Errorf("Expected 2 proxies, got %d", len(proxyInfoList))
	}

	currentIndex := status["current_index"].(int)
	if currentIndex != 0 {
		t.Errorf("Expected current index 0, got %d", currentIndex)
	}

	currentURL := status["current_url"].(string)
	if currentURL == "" {
		t.Error("Current URL should not be empty")
	}
}

func TestProxyListFailover(t *testing.T) {
	// 测试代理故障转移
	proxyList := "socks5://127.0.0.1:9999,socks5://127.0.0.1:1080,http://127.0.0.1:8080"
	dialer, err := NewProxyDialer(proxyList, true)
	if err != nil {
		t.Skipf("Cannot create proxy dialer with list: %v", err)
	}

	ctx := context.Background()

	// 尝试连接（第一个代理会失败，应该自动切换到下一个）
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("All proxies failed (expected in test environment): %v", err)
	} else {
		conn.Close()
		t.Log("At least one proxy succeeded")
	}

	// 检查健康状态
	allHealth := dialer.GetAllProxyHealth()
	healthyCount := 0
	for _, health := range allHealth {
		if health.Healthy {
			healthyCount++
		}
	}
	t.Logf("Healthy proxies: %d/%d", healthyCount, len(allHealth))
}

func TestAllProxiesFailedFallback(t *testing.T) {
	// 测试所有代理都失败时的回退机制
	proxyList := "socks5://127.0.0.1:9999,socks5://127.0.0.1:9998,socks5://127.0.0.1:9997"

	// 测试启用回退
	dialerWithFallback, err := NewProxyDialer(proxyList, true)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer with fallback: %v", err)
	}

	ctx := context.Background()

	// 尝试连接，应该回退到直连
	conn, err := dialerWithFallback.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Direct connection also failed: %v", err)
	} else {
		conn.Close()
		t.Log("Fallback to direct connection succeeded")
	}

	// 测试不启用回退
	dialerNoFallback, err := NewProxyDialer(proxyList, false)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer without fallback: %v", err)
	}

	// 尝试连接，应该失败
	_, err = dialerNoFallback.DialContext(ctx, "tcp", "example.com:80")
	if err == nil {
		t.Error("Should fail when all proxies failed and fallback is disabled")
	} else {
		t.Logf("Expected failure when no fallback: %v", err)
	}
}

func TestDirectConnection(t *testing.T) {
	// 测试 DIRECT 连接
	proxyList := "DIRECT"
	dialer, err := NewProxyDialer(proxyList, false)
	if err != nil {
		t.Fatalf("Failed to create DIRECT dialer: %v", err)
	}

	// 检查当前代理是否为 DIRECT
	currentURL := dialer.GetCurrentProxyURL()
	if currentURL != "direct://direct" {
		t.Errorf("Expected direct://direct, got %s", currentURL)
	}

	// 测试直接连接
	ctx := context.Background()
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Direct connection failed (expected in some environments): %v", err)
	} else {
		conn.Close()
		t.Log("Direct connection succeeded")
	}

	// 检查代理状态
	status := dialer.GetProxyStatus()
	proxyListInterface, ok := status["proxy_list"].([]interface{})
	if !ok {
		t.Error("proxy_list should be a list")
	}

	var proxyInfoList []map[string]interface{}
	for _, item := range proxyListInterface {
		if proxyInfo, ok := item.(map[string]interface{}); ok {
			proxyInfoList = append(proxyInfoList, proxyInfo)
		}
	}
	if len(proxyInfoList) != 1 {
		t.Errorf("Expected 1 proxy (DIRECT), got %d", len(proxyInfoList))
	}

	proxyInfo := proxyInfoList[0]
	if !proxyInfo["is_direct"].(bool) {
		t.Error("Proxy should be marked as direct")
	}
}

func TestProxyListWithDirect(t *testing.T) {
	// 测试包含 DIRECT 的代理列表
	proxyList := "socks5://127.0.0.1:9999,http://127.0.0.1:8080,DIRECT"
	dialer, err := NewProxyDialer(proxyList, false)
	if err != nil {
		t.Fatalf("Failed to create proxy dialer with DIRECT: %v", err)
	}

	// 检查是否有3个代理节点
	allHealth := dialer.GetAllProxyHealth()
	if len(allHealth) != 3 {
		t.Errorf("Expected 3 proxy nodes (2 proxies + DIRECT), got %d", len(allHealth))
	}

	// 模拟前两个代理失败
	dialer.proxyNodes[0].Healthy = false
	dialer.proxyNodes[1].Healthy = false

	// 寻找健康的代理，应该选择 DIRECT
	node := dialer.findHealthyProxy()
	if node == nil || !node.IsDirect {
		t.Error("Should select DIRECT when other proxies are unhealthy")
	}

	// 测试连接，应该通过直连
	ctx := context.Background()
	conn, err := dialer.DialContext(ctx, "tcp", "example.com:80")
	if err != nil {
		t.Logf("Direct connection failed: %v", err)
	} else {
		conn.Close()
		t.Log("Connection succeeded via DIRECT")
	}
}

func TestDialFallbackEquivalent(t *testing.T) {
	// 测试 dialfallback 参数与添加 DIRECT 的等价性

	// 方式1：使用 dialfallback
	dialer1, err := NewProxyDialer("socks5://127.0.0.1:9999", true)
	if err != nil {
		t.Fatalf("Failed to create dialer with fallback: %v", err)
	}

	// 方式2：手动添加 DIRECT
	dialer2, err := NewProxyDialer("socks5://127.0.0.1:9999,DIRECT", false)
	if err != nil {
		t.Fatalf("Failed to create dialer with DIRECT: %v", err)
	}

	// 两者应该有相同数量的代理节点
	health1 := dialer1.GetAllProxyHealth()
	health2 := dialer2.GetAllProxyHealth()

	if len(health1) != len(health2) {
		t.Errorf("Proxy counts should be equal: %d vs %d", len(health1), len(health2))
	}

	// 都应该包含一个 DIRECT 节点
	hasDirect1 := false
	hasDirect2 := false

	status1 := dialer1.GetProxyStatus()
	status2 := dialer2.GetProxyStatus()

	proxyListInterface1 := status1["proxy_list"].([]interface{})
	proxyListInterface2 := status2["proxy_list"].([]interface{})

	var proxyList1 []map[string]interface{}
	for _, item := range proxyListInterface1 {
		if proxyInfo, ok := item.(map[string]interface{}); ok {
			proxyList1 = append(proxyList1, proxyInfo)
		}
	}

	var proxyList2 []map[string]interface{}
	for _, item := range proxyListInterface2 {
		if proxyInfo, ok := item.(map[string]interface{}); ok {
			proxyList2 = append(proxyList2, proxyInfo)
		}
	}

	for _, p := range proxyList1 {
		if p["is_direct"].(bool) {
			hasDirect1 = true
			break
		}
	}

	for _, p := range proxyList2 {
		if p["is_direct"].(bool) {
			hasDirect2 = true
			break
		}
	}

	if !hasDirect1 {
		t.Error("Dialer with fallback should have DIRECT node")
	}
	if !hasDirect2 {
		t.Error("Dialer with explicit DIRECT should have DIRECT node")
	}
}

func TestProxyListValidation(t *testing.T) {
	tests := []struct {
		name        string
		proxyList   string
		expectError bool
	}{
		{
			name:        "Valid proxy list",
			proxyList:   "socks5://127.0.0.1:1080,http://127.0.0.1:8080",
			expectError: false,
		},
		{
			name:        "Single proxy",
			proxyList:   "socks5://127.0.0.1:1080",
			expectError: false,
		},
		{
			name:        "Proxy list with DIRECT",
			proxyList:   "socks5://127.0.0.1:1080,http://127.0.0.1:8080,DIRECT",
			expectError: false,
		},
		{
			name:        "Only DIRECT",
			proxyList:   "DIRECT",
			expectError: false,
		},
		{
			name:        "Empty list",
			proxyList:   "",
			expectError: true,
		},
		{
			name:        "Invalid proxy in list",
			proxyList:   "socks5://127.0.0.1:1080,invalid-url",
			expectError: true,
		},
		{
			name:        "Unsupported scheme",
			proxyList:   "ftp://127.0.0.1:21",
			expectError: true,
		},
		{
			name:        "List with spaces",
			proxyList:   "socks5://127.0.0.1:1080 , http://127.0.0.1:8080",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProxyDialer(tt.proxyList, true)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// UDP support is tested in TestSupportedNetworks function
