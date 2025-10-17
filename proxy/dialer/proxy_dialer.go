package dialer

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

// ProxyNode 单个代理节点
type ProxyNode struct {
	URL           *url.URL
	Dialer        interface{}
	Healthy       bool
	LastCheckTime time.Time
	FailCount     int
	Index         int  // 在列表中的原始位置，用于优先级回切
	IsDirect      bool // 是否为直连
}

// ProxyDialer 代理拨号器，支持 HTTP 和 SOCKS5 代理列表
type ProxyDialer struct {
	dialer      net.Dialer
	proxyNodes  []*ProxyNode
	currentNode int
	fallback    bool

	// 动态回退相关
	mu               sync.RWMutex
	lastCheckTime    time.Time
	checkInterval    time.Duration
	consecutiveFails int

	// 连接超时配置
	connectTimeout time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration

	// 自定义监测配置
	healthCheckURLs  []string
	maxCheckTimeout  time.Duration
	successThreshold int

	// 数据传输感知
	lastSuccessTime  time.Time
	dataTransferIdle time.Duration
}

// ProxyHealth 代理健康状态
type ProxyHealth struct {
	Healthy   bool
	LastCheck time.Time
	FailCount int
}

// NewProxyDialer 创建新的代理拨号器
func NewProxyDialer(dialURL string, fallback bool) (*ProxyDialer, error) {
	if dialURL == "" {
		return nil, fmt.Errorf("dial URL cannot be empty")
	}

	// 解析代理 URL 列表
	proxyURLs := strings.Split(dialURL, ",")
	var proxyNodes []*ProxyNode

	for _, proxyURLStr := range proxyURLs {
		proxyURLStr = strings.TrimSpace(proxyURLStr)
		if proxyURLStr == "" {
			continue
		}

		// 特殊处理 DIRECT
		var proxyURL *url.URL
		if strings.ToUpper(proxyURLStr) == "DIRECT" {
			proxyURL = &url.URL{Scheme: "direct", Host: "direct"}
		} else {
			// 解析代理 URL
			parsedURL, err := url.Parse(proxyURLStr)
			if err != nil {
				return nil, fmt.Errorf("invalid proxy URL %s: %w", proxyURLStr, err)
			}
			proxyURL = parsedURL

			// 检查支持的代理协议
			scheme := strings.ToLower(proxyURL.Scheme)
			if scheme != "http" && scheme != "https" && scheme != "socks5" && scheme != "direct" {
				return nil, fmt.Errorf("unsupported proxy scheme: %s, only http, https, socks5 and direct are supported", scheme)
			}
		}

		// 创建代理节点
		scheme := strings.ToLower(proxyURL.Scheme)
		node := &ProxyNode{
			URL:       proxyURL,
			Healthy:   true, // 初始假设健康
			FailCount: 0,
			Index:     len(proxyNodes), // 记录原始位置（从 0 开始）
			IsDirect:  scheme == "direct",
		}

		proxyNodes = append(proxyNodes, node)
	}

	// 如果启用了回退，在代理列表末尾添加 DIRECT
	if fallback {
		directNode := &ProxyNode{
			URL:       &url.URL{Scheme: "direct", Host: "direct"},
			Healthy:   true,
			FailCount: 0,
			Index:     len(proxyNodes),
			IsDirect:  true,
		}
		proxyNodes = append(proxyNodes, directNode)
	}

	if len(proxyNodes) == 0 {
		return nil, fmt.Errorf("no valid proxy URLs provided")
	}

	pd := &ProxyDialer{
		dialer: net.Dialer{
			Timeout: time.Second * 30,
		},
		proxyNodes:     proxyNodes,
		currentNode:    0,
		fallback:       fallback, // 保留参数用于兼容性，但实际功能由 DIRECT 节点实现
		connectTimeout: time.Second * 30,
		readTimeout:    time.Second * 60,
		writeTimeout:   time.Second * 60,
	}

	// 初始化所有代理拨号器
	err := pd.initProxyDialers()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize proxy dialers: %w", err)
	}

	pd.checkInterval = time.Second * 30 // 默认 30 秒检查一次

	// 初始化默认监测配置
	pd.healthCheckURLs = []string{
		"https://cp.cloudflare.com/",
		"https://connectivitycheck.gstatic.com/generate_204",
		"http://wifi.vivo.com.cn/generate_204",
		"http://www.google.com/generate_204",
	}
	pd.maxCheckTimeout = time.Second * 10
	pd.successThreshold = 1 // 只需要一个成功即可

	// 初始化数据传输感知
	pd.lastSuccessTime = time.Now()
	pd.dataTransferIdle = time.Minute * 5 // 5 分钟无数据传输才进行健康检查

	return pd, nil
}

// initProxyDialers 初始化所有代理拨号器
func (pd *ProxyDialer) initProxyDialers() error {
	for _, node := range pd.proxyNodes {
		if node.IsDirect {
			// 直连节点
			node.Dialer = &pd.dialer
			continue
		}

		scheme := strings.ToLower(node.URL.Scheme)

		switch scheme {
		case "http", "https":
			// HTTP 代理
			node.Dialer = &HTTPProxyDialer{
				proxyURL: node.URL,
				dialer:   pd.dialer,
			}
		case "socks5":
			// SOCKS5 代理
			auth := &proxy.Auth{}
			if node.URL.User != nil {
				auth.User = node.URL.User.Username()
				auth.Password, _ = node.URL.User.Password()
			}

			socksDialer, err := proxy.SOCKS5("tcp", node.URL.Host, auth, &pd.dialer)
			if err != nil {
				return fmt.Errorf("failed to create SOCKS5 dialer for %s: %w", node.URL.String(), err)
			}
			node.Dialer = socksDialer
		default:
			return fmt.Errorf("unsupported proxy scheme: %s, only http, https, socks5 and direct are supported", scheme)
		}
	}

	return nil
}

// getCurrentNode 获取当前代理节点
func (pd *ProxyDialer) getCurrentNode() *ProxyNode {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	if pd.currentNode >= len(pd.proxyNodes) {
		pd.currentNode = 0
	}

	return pd.proxyNodes[pd.currentNode]
}

// switchToNextProxy 切换到下一个可用的代理
func (pd *ProxyDialer) switchToNextProxy() bool {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// 标记当前节点为不健康
	if pd.currentNode >= len(pd.proxyNodes) {
		pd.currentNode = 0
	}

	if pd.currentNode < len(pd.proxyNodes) {
		pd.proxyNodes[pd.currentNode].Healthy = false
		pd.proxyNodes[pd.currentNode].FailCount++
		pd.proxyNodes[pd.currentNode].LastCheckTime = time.Now()
	}

	// 寻找下一个健康的代理
	for i := 1; i <= len(pd.proxyNodes); i++ {
		nextIndex := (pd.currentNode + i) % len(pd.proxyNodes)
		if pd.proxyNodes[nextIndex].Healthy {
			pd.currentNode = nextIndex
			return true
		}
	}

	// 没有找到健康的代理，但不要重置状态，让健康检查来处理
	return false
}

// findHealthyProxy 寻找健康的代理（优先级：前面代理 > 当前代理 > 后面代理）
func (pd *ProxyDialer) findHealthyProxy() *ProxyNode {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// 按原始顺序寻找健康的代理（优先前面的代理）
	for _, node := range pd.proxyNodes {
		if node.Healthy {
			pd.currentNode = node.Index
			return node
		}
	}

	// 如果没有健康的代理，返回第一个（将尝试所有代理）
	if len(pd.proxyNodes) > 0 {
		pd.currentNode = 0
		return pd.proxyNodes[0]
	}

	return nil
}

// DialContext 实现网络拨号
func (pd *ProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// 支持 TCP 和 UDP
	if network != "tcp" && network != "tcp4" && network != "tcp6" &&
		network != "udp" && network != "udp4" && network != "udp6" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	// 如果没有配置代理，直接连接
	if len(pd.proxyNodes) == 0 {
		return pd.dialDirect(ctx, network, address)
	}

	// 尝试通过代理列表连接
	return pd.dialViaProxyList(ctx, network, address)
}

// shouldUseProxy 判断是否应该使用代理
func (pd *ProxyDialer) shouldUseProxy() bool {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	// 检查当前代理是否健康
	currentNode := pd.getCurrentNode()
	if currentNode != nil && currentNode.Healthy {
		return true
	}

	// 如果代理不健康，检查是否到了重新检查的时间
	// 优先检查是否有最近的数据传输成功
	if time.Since(pd.lastSuccessTime) < pd.dataTransferIdle {
		// 最近有数据传输成功，说明代理可能恢复，尝试使用
		pd.mu.RUnlock()
		pd.checkProxyHealth()
		pd.mu.RLock()
		currentNode := pd.getCurrentNode()
		return currentNode != nil && currentNode.Healthy
	}

	if time.Since(pd.lastCheckTime) > pd.checkInterval {
		pd.mu.RUnlock()
		pd.checkProxyHealth()
		pd.mu.RLock()
		currentNode := pd.getCurrentNode()
		return currentNode != nil && currentNode.Healthy
	}

	currentNode = pd.getCurrentNode()
	return currentNode != nil && currentNode.Healthy
}

// dialViaProxyList 通过代理列表连接
func (pd *ProxyDialer) dialViaProxyList(ctx context.Context, network, address string) (net.Conn, error) {
	maxAttempts := len(pd.proxyNodes)
	if maxAttempts == 0 {
		// 如果没有代理，直接连接
		return pd.dialDirect(ctx, network, address)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 寻找健康的代理
		node := pd.findHealthyProxy()
		if node == nil {
			break
		}

		// 尝试通过当前代理连接
		conn, err := pd.dialViaNode(ctx, network, address, node)
		if err == nil {
			// 连接成功，记录成功并返回
			pd.recordNodeSuccess(node)
			return conn, nil
		}

		// 连接失败，记录失败并切换到下一个代理
		pd.recordNodeFailure(node)
		if !pd.switchToNextProxy() {
			// 没有更多可用的代理
			break
		}
	}

	// 所有代理都失败了，如果启用回退，尝试直接连接
	if pd.fallback {
		return pd.dialDirect(ctx, network, address)
	}

	return nil, fmt.Errorf("all proxy nodes failed")
}

// dialViaNode 通过指定代理节点连接
func (pd *ProxyDialer) dialViaNode(ctx context.Context, network, address string, node *ProxyNode) (net.Conn, error) {
	if node.IsDirect {
		// 直连
		if network == "tcp" || network == "tcp4" || network == "tcp6" {
			conn, err := pd.dialer.DialContext(ctx, network, address)
			if err == nil {
				// 设置超时
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.SetReadDeadline(time.Now().Add(pd.readTimeout))
					tcpConn.SetWriteDeadline(time.Now().Add(pd.writeTimeout))
				}
				return &MonitoredConn{Conn: conn, dialer: pd}, nil
			}
			return conn, err
		} else if network == "udp" || network == "udp4" || network == "udp6" {
			conn, err := net.DialUDP(network, nil, resolveUDPAddr(network, address))
			if err == nil {
				conn.SetReadDeadline(time.Now().Add(pd.readTimeout))
				conn.SetWriteDeadline(time.Now().Add(pd.writeTimeout))
			}
			return conn, err
		}
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	if network == "tcp" || network == "tcp4" || network == "tcp6" {
		conn, err := pd.dialTCPViaNode(ctx, network, address, node)
		if err == nil {
			// 包装连接以监控数据传输
			return &MonitoredConn{Conn: conn, dialer: pd}, nil
		}
		return conn, err
	} else if network == "udp" || network == "udp4" || network == "udp6" {
		conn, err := pd.dialUDPViaNode(ctx, network, address, node)
		if err == nil {
			// UDP 连接也使用 MonitoredConn 包装，记录数据传输
			return &MonitoredConn{Conn: conn, dialer: pd}, nil
		}
		return conn, err
	}
	return nil, fmt.Errorf("unsupported network: %s", network)
}

// dialTCPViaNode 通过指定代理节点连接 TCP
func (pd *ProxyDialer) dialTCPViaNode(ctx context.Context, network, address string, node *ProxyNode) (net.Conn, error) {
	if node.IsDirect {
		// 直连
		return pd.dialer.DialContext(ctx, network, address)
	}

	if contextDialer, ok := node.Dialer.(proxy.ContextDialer); ok {
		return contextDialer.DialContext(ctx, network, address)
	} else if dialer, ok := node.Dialer.(net.Dialer); ok {
		return dialer.Dial(network, address)
	}
	return nil, fmt.Errorf("unsupported proxy dialer type")
}

// dialUDPViaNode 通过指定代理节点连接 UDP
func (pd *ProxyDialer) dialUDPViaNode(ctx context.Context, network, address string, node *ProxyNode) (net.Conn, error) {
	if node.IsDirect {
		// 直连 UDP
		return net.DialUDP(network, nil, resolveUDPAddr(network, address))
	}

	// 只有 SOCKS5 支持 UDP 代理
	if strings.ToLower(node.URL.Scheme) != "socks5" {
		return nil, fmt.Errorf("UDP proxy only supported with SOCKS5")
	}

	// 对于 SOCKS5 UDP，我们需要建立 TCP 连接到代理服务器，然后发送 UDP ASSOCIATE 请求
	tcpConn, err := pd.dialTCPViaNode(ctx, "tcp", node.URL.Host, node)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SOCKS5 proxy for UDP associate: %w", err)
	}

	// 实现 SOCKS5 UDP ASSOCIATE
	udpConn, err := pd.socks5UDPAssociate(ctx, tcpConn, address)
	if err != nil {
		tcpConn.Close()
		return nil, err
	}

	return udpConn, nil
}

// dialDirect 直接连接
func (pd *ProxyDialer) dialDirect(ctx context.Context, network, address string) (net.Conn, error) {
	if network == "tcp" || network == "tcp4" || network == "tcp6" {
		return pd.dialer.DialContext(ctx, network, address)
	} else if network == "udp" || network == "udp4" || network == "udp6" {
		return net.DialUDP(network, nil, resolveUDPAddr(network, address))
	}
	return nil, fmt.Errorf("unsupported network: %s", network)
}

// recordNodeSuccess 记录代理节点成功
func (pd *ProxyDialer) recordNodeSuccess(node *ProxyNode) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	node.Healthy = true
	node.LastCheckTime = time.Now()
	node.FailCount = 0
	pd.lastSuccessTime = time.Now()
	pd.consecutiveFails = 0
}

// recordDataTransferSuccess 记录数据传输成功
func (pd *ProxyDialer) recordDataTransferSuccess() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.lastSuccessTime = time.Now()
	pd.consecutiveFails = 0

	// 标记当前节点为健康
	currentNode := pd.getCurrentNode()
	if currentNode != nil {
		currentNode.Healthy = true
		currentNode.LastCheckTime = time.Now()
		currentNode.FailCount = 0
	}
}

// recordNodeFailure 记录代理节点失败
func (pd *ProxyDialer) recordNodeFailure(node *ProxyNode) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	node.FailCount++
	node.LastCheckTime = time.Now()

	// 连续失败 3 次以上，标记节点为不健康
	if node.FailCount >= 3 {
		node.Healthy = false
	}

	pd.consecutiveFails++
}

// recordDataTransferFailure 记录数据传输失败
func (pd *ProxyDialer) recordDataTransferFailure() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	pd.consecutiveFails++

	// 标记当前节点为不健康
	currentNode := pd.getCurrentNode()
	if currentNode != nil {
		currentNode.Healthy = false
		currentNode.FailCount++
		currentNode.LastCheckTime = time.Now()
	}
}

// checkProxyHealth 检查代理健康状态（智能回切）
func (pd *ProxyDialer) checkProxyHealth() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	// 如果最近有数据传输成功，跳过健康检查
	if time.Since(pd.lastSuccessTime) < pd.dataTransferIdle {
		return
	}

	// 按优先级顺序检查代理节点（前面的代理优先）
	// 如果前面的代理恢复了，优先切换回去
	hasHealthyProxy := false

	for _, node := range pd.proxyNodes {
		if pd.checkNodeHealth(node) {
			node.Healthy = true
			node.FailCount = 0
			hasHealthyProxy = true

			// 如果这个代理比当前代理更靠前，切换过去
			if node.Index < pd.currentNode || !pd.proxyNodes[pd.currentNode].Healthy {
				pd.currentNode = node.Index
			}
		} else {
			node.Healthy = false
			node.FailCount++
		}
		node.LastCheckTime = time.Now()
	}

	// 如果没有任何健康的代理，重置当前索引到第一个
	if !hasHealthyProxy {
		pd.currentNode = 0
	}

	pd.lastCheckTime = time.Now()
}

// checkNodeHealth 检查单个代理节点的健康状态
func (pd *ProxyDialer) checkNodeHealth(node *ProxyNode) bool {
	if node.IsDirect {
		// 直连总是被认为是健康的（除非网络本身有问题）
		return true
	}

	// 使用自定义的 URL 列表来检查代理健康状态
	successCount := 0

	for _, testURL := range pd.healthCheckURLs {
		if pd.checkURLHealthWithNode(testURL, node) {
			successCount++
			if successCount >= pd.successThreshold {
				return true
			}
		}
	}

	return false
}

// checkURLHealthWithNode 通过指定代理节点检查 URL 健康状态
func (pd *ProxyDialer) checkURLHealthWithNode(testURL string, node *ProxyNode) bool {
	if node.IsDirect {
		// 对于直连，简单测试网络连接即可
		ctx, cancel := context.WithTimeout(context.Background(), pd.maxCheckTimeout)
		defer cancel()

		// 解析 URL
		parsedURL, err := url.Parse(testURL)
		if err != nil {
			return false
		}

		// 直接连接
		host := parsedURL.Host
		if parsedURL.Port() == "" {
			if parsedURL.Scheme == "https" {
				host += ":443"
			} else {
				host += ":80"
			}
		}

		conn, err := pd.dialer.DialContext(ctx, "tcp", host)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), pd.maxCheckTimeout)
	defer cancel()

	// 解析 URL
	parsedURL, err := url.Parse(testURL)
	if err != nil {
		return false
	}

	// 通过代理连接
	host := parsedURL.Host
	if parsedURL.Port() == "" {
		if parsedURL.Scheme == "https" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	conn, err := pd.dialTCPViaNode(ctx, "tcp", host, node)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 发送 HTTP 请求
	if parsedURL.Scheme == "https" {
		// 简单的 HTTPS 检查
		tlsConn := tls.Client(conn, &tls.Config{
			InsecureSkipVerify: true,
		})
		if err := tlsConn.Handshake(); err != nil {
			return false
		}
		conn = tlsConn
	}

	// 发送简单的 HTTP 请求
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: anytls-proxy-check\r\nConnection: close\r\n\r\n",
		parsedURL.RequestURI(), parsedURL.Host)

	_, err = conn.Write([]byte(req))
	if err != nil {
		return false
	}

	// 读取响应
	response := make([]byte, 512)
	_, err = conn.Read(response)
	if err != nil {
		return false
	}

	// 检查响应状态码（简单的检查）
	responseStr := string(response)
	return strings.Contains(responseStr, "200") || strings.Contains(responseStr, "204") ||
		strings.Contains(responseStr, "Connection established") || len(response) > 0
}

// checkURLHealth 检查单个 URL 的健康状态
func (pd *ProxyDialer) checkURLHealth(testURL string) bool {
	// 使用当前代理节点检查
	currentNode := pd.getCurrentNode()
	if currentNode == nil {
		return false
	}
	return pd.checkURLHealthWithNode(testURL, currentNode)
}

// SetHealthCheckConfig 设置健康检查配置
func (pd *ProxyDialer) SetHealthCheckConfig(urls []string, interval time.Duration, timeout time.Duration, threshold int) {
	pd.SetHealthCheckConfigAdvanced(urls, interval, timeout, threshold, time.Minute*5)
}

// SetHealthCheckConfigAdvanced 设置高级健康检查配置
func (pd *ProxyDialer) SetHealthCheckConfigAdvanced(urls []string, interval time.Duration, timeout time.Duration, threshold int, dataIdle time.Duration) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if len(urls) > 0 {
		pd.healthCheckURLs = urls
	}
	if interval > 0 {
		pd.checkInterval = interval
	}
	if timeout > 0 {
		pd.maxCheckTimeout = timeout
	}
	if threshold > 0 {
		pd.successThreshold = threshold
	}
	if dataIdle > 0 {
		pd.dataTransferIdle = dataIdle
	}
}

// GetProxyHealth 获取代理健康状态
func (pd *ProxyDialer) GetProxyHealth() ProxyHealth {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	// 找到当前使用的代理节点
	currentNode := pd.getCurrentNode()
	if currentNode == nil {
		return ProxyHealth{
			Healthy:   false,
			LastCheck: pd.lastCheckTime,
			FailCount: pd.consecutiveFails,
		}
	}

	return ProxyHealth{
		Healthy:   currentNode.Healthy,
		LastCheck: currentNode.LastCheckTime,
		FailCount: currentNode.FailCount,
	}
}

// GetAllProxyHealth 获取所有代理节点的健康状态
func (pd *ProxyDialer) GetAllProxyHealth() []ProxyHealth {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	var healthList []ProxyHealth
	for _, node := range pd.proxyNodes {
		healthList = append(healthList, ProxyHealth{
			Healthy:   node.Healthy,
			LastCheck: node.LastCheckTime,
			FailCount: node.FailCount,
		})
	}

	return healthList
}

// GetCurrentProxyURL 获取当前使用的代理 URL
func (pd *ProxyDialer) GetCurrentProxyURL() string {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	if pd.currentNode < len(pd.proxyNodes) {
		return pd.proxyNodes[pd.currentNode].URL.String()
	}
	return ""
}

// GetHealthCheckConfig 获取当前健康检查配置
func (pd *ProxyDialer) GetHealthCheckConfig() (urls []string, interval time.Duration, timeout time.Duration, threshold int) {
	urls, interval, timeout, threshold, _ = pd.GetHealthCheckConfigAdvanced()
	return
}

// GetProxyStatus 获取详细的代理状态信息
func (pd *ProxyDialer) GetProxyStatus() map[string]interface{} {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	status := make(map[string]interface{})

	var proxyList []interface{}
	for _, node := range pd.proxyNodes {
		proxyInfo := map[string]interface{}{
			"url":        node.URL.String(),
			"healthy":    node.Healthy,
			"fail_count": node.FailCount,
			"last_check": node.LastCheckTime.Format(time.RFC3339),
			"index":      node.Index,
			"current":    pd.currentNode == node.Index,
			"is_direct":  node.IsDirect,
		}
		proxyList = append(proxyList, proxyInfo)
	}

	status["proxy_list"] = proxyList
	status["current_index"] = pd.currentNode
	status["current_url"] = pd.GetCurrentProxyURL()
	status["fallback_enabled"] = pd.fallback
	status["last_success_time"] = pd.lastSuccessTime.Format(time.RFC3339)
	status["check_interval"] = pd.checkInterval.String()
	status["connect_timeout"] = pd.connectTimeout.String()
	status["read_timeout"] = pd.readTimeout.String()
	status["write_timeout"] = pd.writeTimeout.String()

	return status
}

// SetTimeouts 设置连接超时
func (pd *ProxyDialer) SetTimeouts(connectTimeout, readTimeout, writeTimeout time.Duration) {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if connectTimeout > 0 {
		pd.connectTimeout = connectTimeout
	}
	if readTimeout > 0 {
		pd.readTimeout = readTimeout
	}
	if writeTimeout > 0 {
		pd.writeTimeout = writeTimeout
	}
}

// GetTimeouts 获取当前超时配置
func (pd *ProxyDialer) GetTimeouts() (connectTimeout, readTimeout, writeTimeout time.Duration) {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	return pd.connectTimeout, pd.readTimeout, pd.writeTimeout
}

// GetHealthCheckConfigAdvanced 获取当前高级健康检查配置
func (pd *ProxyDialer) GetHealthCheckConfigAdvanced() (urls []string, interval time.Duration, timeout time.Duration, threshold int, dataIdle time.Duration) {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	// 返回副本以避免并发修改
	urls = make([]string, len(pd.healthCheckURLs))
	copy(urls, pd.healthCheckURLs)
	interval = pd.checkInterval
	timeout = pd.maxCheckTimeout
	threshold = pd.successThreshold
	dataIdle = pd.dataTransferIdle

	return
}

// MonitoredConn 监控数据传输的连接包装器
type MonitoredConn struct {
	net.Conn
	dialer *ProxyDialer
}

// Read 实现 net.Conn 的 Read 方法，监控数据读取
func (mc *MonitoredConn) Read(b []byte) (n int, err error) {
	n, err = mc.Conn.Read(b)
	if n > 0 && err == nil {
		// 成功读取数据，记录传输成功
		mc.dialer.recordDataTransferSuccess()
	} else if err != nil {
		// 读取失败，记录传输失败
		mc.dialer.recordDataTransferFailure()
	}
	return n, err
}

// Write 实现 net.Conn 的 Write 方法，监控数据写入
func (mc *MonitoredConn) Write(b []byte) (n int, err error) {
	n, err = mc.Conn.Write(b)
	if n > 0 && err == nil {
		// 成功写入数据，记录传输成功
		mc.dialer.recordDataTransferSuccess()
	} else if err != nil {
		// 写入失败，记录传输失败
		mc.dialer.recordDataTransferFailure()
	}
	return n, err
}

// MonitoredPacketConn 监控数据传输的 PacketConn 包装器
type MonitoredPacketConn struct {
	net.PacketConn
	dialer *ProxyDialer
}

// ReadFrom 实现 net.PacketConn 的 ReadFrom 方法，监控数据读取
func (mpc *MonitoredPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, addr, err = mpc.PacketConn.ReadFrom(p)
	if n > 0 && err == nil {
		// 成功读取数据，记录传输成功
		mpc.dialer.recordDataTransferSuccess()
	} else if err != nil {
		// 读取失败，记录传输失败
		mpc.dialer.recordDataTransferFailure()
	}
	return n, addr, err
}

// WriteTo 实现 net.PacketConn 的 WriteTo 方法，监控数据写入
func (mpc *MonitoredPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	n, err = mpc.PacketConn.WriteTo(p, addr)
	if n > 0 && err == nil {
		// 成功写入数据，记录传输成功
		mpc.dialer.recordDataTransferSuccess()
	} else if err != nil {
		// 写入失败，记录传输失败
		mpc.dialer.recordDataTransferFailure()
	}
	return n, err
}

// resolveUDPAddr 解析 UDP 地址
func resolveUDPAddr(network, address string) *net.UDPAddr {
	if addr, err := net.ResolveUDPAddr(network, address); err == nil {
		return addr
	}
	// 如果解析失败，尝试简单的分割
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return &net.UDPAddr{Port: 53} // 默认 DNS 端口
	}
	if port == "" {
		port = "53"
	}
	return &net.UDPAddr{IP: net.ParseIP(host), Port: 0} // 让系统选择端口
}

// CreatePacketConn 创建 PacketConn 连接
func CreatePacketConn(network, address string) (net.PacketConn, error) {
	if network == "udp" || network == "udp4" || network == "udp6" {
		return net.ListenPacket(network, address)
	}
	return nil, fmt.Errorf("unsupported network for PacketConn: %s", network)
}

// Dial 实现 net.Dialer 接口的 Dial 方法
func (pd *ProxyDialer) Dial(network, address string) (net.Conn, error) {
	return pd.DialContext(context.Background(), network, address)
}

// socks5UDPAssociate 实现 SOCKS5 UDP ASSOCIATE
func (pd *ProxyDialer) socks5UDPAssociate(ctx context.Context, tcpConn net.Conn, targetAddr string) (net.Conn, error) {
	// SOCKS5 UDP ASSOCIATE 实现
	// 这里需要实现完整的 SOCKS5 UDP ASSOCIATE 协议

	// 1. 发送 SOCKS5 握手
	_, err := tcpConn.Write([]byte{0x05, 0x01, 0x00}) // VER, NMETHODS, METHOD
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 handshake failed: %w", err)
	}

	// 2. 读取响应
	response := make([]byte, 2)
	_, err = tcpConn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 handshake response failed: %w", err)
	}

	if response[0] != 0x05 || response[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 handshake not accepted")
	}

	// 3. 发送 UDP ASSOCIATE 请求
	// 构造请求：VER(1) + CMD(1) + RSV(1) + ATYP(1) + DST.ADDR(var) + DST.PORT(2))
	request := []byte{0x05, 0x03, 0x00, 0x01} // VER, CMD(UDP ASSOCIATE), RSV, ATYP(IPv4)

	// 添加目标地址 0.0.0.0:0 (让代理服务器选择)
	request = append(request, []byte{0x00, 0x00, 0x00, 0x00}...) // 0.0.0.0
	request = append(request, []byte{0x00, 0x00}...)             // port 0

	_, err = tcpConn.Write(request)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 UDP ASSOCIATE request failed: %w", err)
	}

	// 4. 读取 UDP ASSOCIATE 响应
	response = make([]byte, 10) // 最小响应长度
	_, err = tcpConn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 UDP ASSOCIATE response failed: %w", err)
	}

	if response[0] != 0x05 || response[1] != 0x00 {
		return nil, fmt.Errorf("SOCKS5 UDP ASSOCIATE failed: status %d", response[1])
	}

	// 5. 解析 UDP 代理服务器地址
	var udpProxyAddr string
	if response[3] == 0x01 { // IPv4
		udpProxyAddr = fmt.Sprintf("%d.%d.%d.%d:%d",
			response[4], response[5], response[6], response[7],
			uint16(response[8])<<8|uint16(response[9]))
	} else {
		return nil, fmt.Errorf("unsupported address type in UDP ASSOCIATE response")
	}

	// 6. 创建 UDP 连接到代理服务器
	udpConn, err := net.Dial("udp", udpProxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial UDP proxy: %w", err)
	}

	// 返回一个特殊的连接，它会处理 SOCKS5 UDP 包格式
	return &Socks5UDPConn{
		udpConn:    udpConn.(*net.UDPConn),
		tcpConn:    tcpConn,
		targetAddr: targetAddr,
	}, nil
}

// Socks5UDPConn SOCKS5 UDP 连接包装器
type Socks5UDPConn struct {
	udpConn    *net.UDPConn
	tcpConn    net.Conn
	targetAddr string
}

// Read 实现 net.Conn 的 Read 方法
func (c *Socks5UDPConn) Read(b []byte) (n int, err error) {
	// 读取 UDP 数据包
	n, _, err = c.udpConn.ReadFromUDP(b)
	if err != nil {
		return n, err
	}

	// 解析 SOCKS5 UDP 包格式并提取实际数据
	// SOCKS5 UDP 包格式：RSV(2) + FRAG(1) + ATYP(1) + DST.ADDR(var) + DST.PORT(2) + DATA(var)
	if n < 10 { // 最小包头长度
		return n, fmt.Errorf("invalid SOCKS5 UDP packet")
	}

	// 跳过 SOCKS5 UDP 包头，返回实际数据
	dataStart := 10   // IPv4 的最小包头长度
	if b[3] == 0x03 { // 域名
		dataStart = 5 + int(b[4]) + 2 // 域名长度 + 端口
	} else if b[3] == 0x04 { // IPv6
		dataStart = 19 // IPv6 地址长度 + 端口
	}

	// 移动数据到缓冲区开头
	copy(b, b[dataStart:n])
	return n - dataStart, nil
}

// Write 实现 net.Conn 的 Write 方法
func (c *Socks5UDPConn) Write(b []byte) (n int, err error) {
	// 构造 SOCKS5 UDP 包
	// RSV(2) + FRAG(1) + ATYP(1) + DST.ADDR(var) + DST.PORT(2) + DATA(var)
	packet := make([]byte, 0, 10+len(b))
	packet = append(packet, 0x00, 0x00) // RSV
	packet = append(packet, 0x00)       // FRAG

	// 添加目标地址
	host, port, err := net.SplitHostPort(c.targetAddr)
	if err != nil {
		return 0, err
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			// IPv4
			packet = append(packet, 0x01) // ATYP
			packet = append(packet, ip.To4()...)
		} else {
			// IPv6
			packet = append(packet, 0x04) // ATYP
			packet = append(packet, ip...)
		}
	} else {
		// 域名
		packet = append(packet, 0x03) // ATYP
		packet = append(packet, byte(len(host)))
		packet = append(packet, []byte(host)...)
	}

	// 添加端口
	portNum, err := net.LookupPort("udp", port)
	if err != nil {
		return 0, err
	}
	packet = append(packet, byte(portNum>>8), byte(portNum&0xff))

	// 添加数据
	packet = append(packet, b...)

	// 发送 UDP 包
	return c.udpConn.Write(packet)
}

// Close 实现 net.Conn 的 Close 方法
func (c *Socks5UDPConn) Close() error {
	c.udpConn.Close()
	return c.tcpConn.Close()
}

// LocalAddr 实现 net.Conn 的 LocalAddr 方法
func (c *Socks5UDPConn) LocalAddr() net.Addr {
	return c.udpConn.LocalAddr()
}

// RemoteAddr 实现 net.Conn 的 RemoteAddr 方法
func (c *Socks5UDPConn) RemoteAddr() net.Addr {
	return c.udpConn.RemoteAddr()
}

// SetDeadline 实现 net.Conn 的 SetDeadline 方法
func (c *Socks5UDPConn) SetDeadline(t time.Time) error {
	return c.udpConn.SetDeadline(t)
}

// SetReadDeadline 实现 net.Conn 的 SetReadDeadline 方法
func (c *Socks5UDPConn) SetReadDeadline(t time.Time) error {
	return c.udpConn.SetReadDeadline(t)
}

// SetWriteDeadline 实现 net.Conn 的 SetWriteDeadline 方法
func (c *Socks5UDPConn) SetWriteDeadline(t time.Time) error {
	return c.udpConn.SetWriteDeadline(t)
}

// HTTPProxyDialer HTTP 代理拨号器实现
type HTTPProxyDialer struct {
	proxyURL *url.URL
	dialer   net.Dialer
}

// Dial 实现 net.Dialer 接口
func (hpd *HTTPProxyDialer) Dial(network, address string) (net.Conn, error) {
	return hpd.DialContext(context.Background(), network, address)
}

// DialContext 实现 HTTP 代理拨号
func (hpd *HTTPProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	// 连接到代理服务器
	conn, err := hpd.dialer.DialContext(ctx, network, hpd.proxyURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy %s: %w", hpd.proxyURL.Host, err)
	}

	// 发送 HTTP CONNECT 请求
	connectReq := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: address},
		Header: make(http.Header),
		Host:   address,
	}

	// 添加代理认证
	if hpd.proxyURL.User != nil {
		username := hpd.proxyURL.User.Username()
		password, _ := hpd.proxyURL.User.Password()
		if username != "" || password != "" {
			connectReq.Header.Set("Proxy-Authorization", "Basic "+basicAuth(username, password))
		}
	}

	// 发送请求
	err = connectReq.Write(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %w", err)
	}

	// 读取响应
	resp, err := http.ReadResponse(bufio.NewReader(conn), connectReq)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("proxy returned status %d", resp.StatusCode)
	}

	return conn, nil
}

// basicAuth 生成基本认证头
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
