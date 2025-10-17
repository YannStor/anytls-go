package simpledialer

import (
	"bufio"
	"context"
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

// ProxyNode 简化的代理节点
type ProxyNode struct {
	URL      *url.URL
	Dialer   proxy.Dialer
	Healthy  bool
	LastUsed time.Time
	Fails    int
	IsDirect bool // 是否为直连
	Index    int  // 在列表中的原始位置，用于优先级回切
}

// SimpleDialer 简化的代理拨号器
type SimpleDialer struct {
	nodes         []*ProxyNode
	current       int
	mu            sync.RWMutex
	checkURL      string
	checkInterval time.Duration
}

// NewSimpleDialer 创建简化代理拨号器
func NewSimpleDialer(proxyList string) (*SimpleDialer, error) {
	if proxyList == "" {
		return nil, fmt.Errorf("proxy list cannot be empty")
	}

	sd := &SimpleDialer{
		checkURL:      "https://www.google.com/generate_204",
		checkInterval: time.Minute * 5,
	}

	// 解析代理列表
	proxyURLs := strings.Split(proxyList, ",")
	for i, proxyURL := range proxyURLs {
		proxyURL = strings.TrimSpace(proxyURL)
		if proxyURL == "" {
			continue
		}

		// 处理 DIRECT
		if strings.ToUpper(proxyURL) == "DIRECT" {
			sd.nodes = append(sd.nodes, &ProxyNode{
				URL:      &url.URL{Scheme: "direct", Host: "direct"},
				Dialer:   &net.Dialer{Timeout: time.Second * 30},
				Healthy:  true,
				IsDirect: true,
				Index:    len(sd.nodes),
			})
			continue
		}

		// 解析代理 URL
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %s: %w", proxyURL, err)
		}

		// 创建拨号器
		var dialer proxy.Dialer
		switch strings.ToLower(parsed.Scheme) {
		case "socks5":
			auth := &proxy.Auth{}
			if parsed.User != nil {
				auth.User = parsed.User.Username()
				auth.Password, _ = parsed.User.Password()
			}
			dialer, err = proxy.SOCKS5("tcp", parsed.Host, auth, &net.Dialer{Timeout: time.Second * 30})
		case "http", "https":
			dialer, err = newHTTPProxyDialer(parsed)
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", parsed.Scheme)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create dialer for %s: %w", proxyURL, err)
		}

		sd.nodes = append(sd.nodes, &ProxyNode{
			URL:      parsed,
			Dialer:   dialer,
			Healthy:  true,
			IsDirect: false,
			Index:    len(sd.nodes),
		})

		// 第一个节点作为当前节点
		if i == 0 {
			sd.current = 0
		}
	}

	if len(sd.nodes) == 0 {
		return nil, fmt.Errorf("no valid proxy nodes")
	}

	// 只有在多个代理或包含 DIRECT 时才启动健康检查
	// 单个代理且无回退时，健康检查没有意义
	if sd.shouldEnableHealthCheck() {
		go sd.healthChecker()
	}

	return sd, nil
}

// DialContext 实现拨号
func (sd *SimpleDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	sd.mu.RLock()
	if len(sd.nodes) == 0 {
		sd.mu.RUnlock()
		return nil, fmt.Errorf("no proxy nodes available")
	}

	// 尝试当前节点
	node := sd.nodes[sd.current]
	sd.mu.RUnlock()

	conn, err := sd.dialViaNode(ctx, node, network, address)
	if err == nil {
		sd.recordSuccess(node)
		return conn, nil
	}

	sd.recordFailure(node)

	// 尝试其他健康节点
	sd.mu.RLock()
	healthyNodes := sd.getHealthyNodes()
	sd.mu.RUnlock()

	for _, node := range healthyNodes {
		if node == sd.nodes[sd.current] {
			continue // 跳过已经尝试过的节点
		}

		conn, err := sd.dialViaNode(ctx, node, network, address)
		if err == nil {
			// 记录切换前的节点
			previousNode := sd.nodes[sd.current]
			sd.recordSuccess(node)
			sd.switchToNode(node)
			// 故障切换日志
			fmt.Printf("[SimpleDialer] Failover: switched from %s to %s\n",
				previousNode.URL.String(), node.URL.String())
			return conn, nil
		}
		sd.recordFailure(node)
	}

	return nil, fmt.Errorf("all proxy nodes failed")
}

// dialViaNode 通过指定节点拨号
func (sd *SimpleDialer) dialViaNode(ctx context.Context, node *ProxyNode, network, address string) (net.Conn, error) {
	if ctxDialer, ok := node.Dialer.(proxy.ContextDialer); ok {
		return ctxDialer.DialContext(ctx, network, address)
	}
	return node.Dialer.Dial(network, address)
}

// getHealthyNodes 获取健康节点列表
func (sd *SimpleDialer) getHealthyNodes() []*ProxyNode {
	var healthy []*ProxyNode
	for _, node := range sd.nodes {
		if node.Healthy {
			healthy = append(healthy, node)
		}
	}
	return healthy
}

// switchToNode 切换到指定节点
func (sd *SimpleDialer) switchToNode(target *ProxyNode) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	for i, node := range sd.nodes {
		if node == target {
			sd.current = i
			break
		}
	}
}

// recordSuccess 记录成功
func (sd *SimpleDialer) recordSuccess(node *ProxyNode) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	node.Healthy = true
	node.Fails = 0
	node.LastUsed = time.Now()
}

// recordFailure 记录失败
func (sd *SimpleDialer) recordFailure(node *ProxyNode) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	node.Fails++
	if node.Fails >= 3 {
		node.Healthy = false
	}
}

// shouldEnableHealthCheck 判断是否应该启用健康检查
func (sd *SimpleDialer) shouldEnableHealthCheck() bool {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	// 如果只有一个代理节点
	if len(sd.nodes) == 1 {
		// 检查是否为 DIRECT（直连不需要健康检查）
		if sd.nodes[0].IsDirect {
			return false
		}
		// 单个非 DIRECT 代理，没有备选项，不需要健康检查
		return false
	}

	// 多个代理或包含 DIRECT 时，需要健康检查
	return true
}

// healthChecker 健康检查器
func (sd *SimpleDialer) healthChecker() {
	ticker := time.NewTicker(sd.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		sd.mu.RLock()
		nodes := make([]*ProxyNode, len(sd.nodes))
		copy(nodes, sd.nodes)
		currentIndex := sd.current
		sd.mu.RUnlock()

		for _, node := range nodes {
			if node.Healthy {
				continue
			}

			// DIRECT 节点不需要健康检查
			if node.IsDirect {
				continue
			}

			if sd.checkNode(node) {
				sd.recordSuccess(node)

				// 智能回切：如果恢复的节点优先级更高，则切换回去
				sd.mu.RLock()
				shouldSwitch := node.Index < currentIndex && !sd.nodes[currentIndex].Healthy
				sd.mu.RUnlock()

				if shouldSwitch {
					sd.switchToNode(node)
					fmt.Printf("[SimpleDialer] Smart recovery: switched back to higher priority proxy %s\n",
						node.URL.String())
				} else {
					fmt.Printf("[SimpleDialer] Health check: proxy %s recovered\n",
						node.URL.String())
				}
			}
		}
	}
}

// checkNode 检查单个节点
func (sd *SimpleDialer) checkNode(node *ProxyNode) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	conn, err := sd.dialViaNode(ctx, node, "tcp", "www.google.com:443")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetCurrentProxy 获取当前代理
func (sd *SimpleDialer) GetCurrentProxy() string {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if sd.current >= len(sd.nodes) {
		return ""
	}
	return sd.nodes[sd.current].URL.String()
}

// GetProxyStatus 获取代理状态
func (sd *SimpleDialer) GetProxyStatus() map[string]interface{} {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	status := make(map[string]interface{})
	var proxies []map[string]interface{}

	for i, node := range sd.nodes {
		proxies = append(proxies, map[string]interface{}{
			"url":       node.URL.String(),
			"healthy":   node.Healthy,
			"fails":     node.Fails,
			"last_used": node.LastUsed.Format(time.RFC3339),
			"current":   i == sd.current,
		})
	}

	status["proxies"] = proxies
	status["current"] = sd.current
	return status
}

// newHTTPProxyDialer 创建 HTTP 代理拨号器
func newHTTPProxyDialer(proxyURL *url.URL) (proxy.Dialer, error) {
	return &httpProxyDialer{proxyURL: proxyURL}, nil
}

// httpProxyDialer HTTP 代理拨号器
type httpProxyDialer struct {
	proxyURL *url.URL
}

func (hpd *httpProxyDialer) Dial(network, address string) (net.Conn, error) {
	return hpd.DialContext(context.Background(), network, address)
}

func (hpd *httpProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: time.Second * 30}
	conn, err := dialer.DialContext(ctx, network, hpd.proxyURL.Host)
	if err != nil {
		return nil, err
	}

	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Host: address},
		Header: make(http.Header),
		Host:   address,
	}

	if hpd.proxyURL.User != nil {
		username := hpd.proxyURL.User.Username()
		password, _ := hpd.proxyURL.User.Password()
		auth := username + ":" + password
		req.Header.Set("Proxy-Authorization", "Basic "+base64Encode(auth))
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("proxy returned status %d", resp.StatusCode)
	}

	return conn, nil
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
