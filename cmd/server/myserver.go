package main

import (
	"anytls/proxy/simpledialer"
	"crypto/tls"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type myServer struct {
	tlsConfig   *tls.Config
	proxyDialer *simpledialer.SimpleDialer
}

func NewMyServer(tlsConfig *tls.Config, dialURL string, dialFallback bool, healthCheckURLs string, healthCheckInterval time.Duration, healthCheckTimeout time.Duration, healthCheckThreshold int, dataTransferIdle time.Duration, connectTimeout time.Duration, readTimeout time.Duration, writeTimeout time.Duration) *myServer {
	s := &myServer{
		tlsConfig: tlsConfig,
	}

	// 如果配置了出站代理，初始化代理拨号器
	if dialURL != "" {
		proxyDialer, err := simpledialer.NewSimpleDialer(dialURL)
		if err != nil {
			logrus.Fatalln("failed to create proxy dialer:", err)
		}
		s.proxyDialer = proxyDialer

		logrus.Infoln("[Server] Using outbound proxy:", dialURL)
		logrus.Infoln("[Server] Proxy list:", proxyDialer.GetCurrentProxy())
	} else {
		logrus.Infoln("[Server] Using direct outbound connection")
	}

	return s
}

// parseHealthCheckURLs 解析健康检查 URL 列表
func (s *myServer) parseHealthCheckURLs(urlsStr string) []string {
	if urlsStr == "" {
		// 返回默认 URL 列表
		return []string{
			"https://cp.cloudflare.com/",
			"https://connectivitycheck.gstatic.com/generate_204",
			"http://wifi.vivo.com.cn/generate_204",
			"http://www.google.com/generate_204",
		}
	}

	// 解析用户提供的 URL 列表
	urls := strings.Split(urlsStr, ",")
	var result []string
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url != "" {
			result = append(result, url)
		}
	}

	if len(result) == 0 {
		// 如果用户提供的列表为空，使用默认列表
		return []string{
			"https://cp.cloudflare.com/",
			"https://connectivitycheck.gstatic.com/generate_204",
			"http://wifi.vivo.com.cn/generate_204",
			"http://www.google.com/generate_204",
		}
	}

	return result
}

// GetDialer 获取拨号器，优先使用代理拨号器
func (s *myServer) GetDialer() interface{} {
	if s.proxyDialer != nil {
		return s.proxyDialer
	}
	// 返回默认的系统拨号器
	return &net.Dialer{
		Timeout: time.Second * 30,
	}
}
