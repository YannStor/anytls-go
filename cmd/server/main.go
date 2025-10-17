package main

import (
	"anytls/proxy/padding"
	"anytls/util"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

var passwordSha256 []byte
var connectionCount int64

// 版本信息（构建时注入）
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	listen := flag.String("l", "0.0.0.0:15000", "server listen port")
	password := flag.String("p", "thisismynetwork", "password")
	paddingScheme := flag.String("padding-scheme", "", "padding-scheme")
	sni := flag.String("n", "liveplay.wemeet.tencent.com", "TLS serverName")
	dial := flag.String("dial", "", "outbound proxy URL(s), comma-separated for multiple proxies (e.g., socks5://user:pass@127.0.0.1:1080,http://user:pass@127.0.0.1:8080,DIRECT)")
	dialFallback := flag.Bool("dialfallback", false, "fallback to direct connection when proxy fails (equivalent to adding DIRECT at the end of dial list) (default: false)")

	// 健康检查配置参数
	healthCheckURLs := flag.String("health-urls", "", "comma-separated list of health check URLs (default: https://cp.cloudflare.com/,https://connectivitycheck.gstatic.com/generate_204,http://wifi.vivo.com.cn/generate_204,http://www.google.com/generate_204)")
	healthCheckInterval := flag.Duration("health-interval", 0, "health check interval (default: 30s)")
	healthCheckTimeout := flag.Duration("health-timeout", 0, "health check timeout (default: 10s)")
	healthCheckThreshold := flag.Int("health-threshold", 0, "number of successful health checks required (default: 1)")
	dataTransferIdle := flag.Duration("data-idle", 0, "data transfer idle time before health check (default: 5m)")
	showVersion := flag.Bool("version", false, "show version information")

	// 连接超时配置
	connectTimeout := flag.Duration("connect-timeout", 0, "connection timeout (default: 30s)")
	readTimeout := flag.Duration("read-timeout", 0, "read timeout (default: 60s)")
	writeTimeout := flag.Duration("write-timeout", 0, "write timeout (default: 60s)")

	flag.Parse()

	// 显示版本信息
	if *showVersion {
		fmt.Printf("AnyTLS Server %s\n", Version)
		fmt.Printf("Build Time: %s\n", BuildTime)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		os.Exit(0)
	}

	if *password == "" {
		logrus.Fatalln("Password is required. Please set -p parameter")
	}
	if *paddingScheme != "" {
		if f, err := os.Open(*paddingScheme); err == nil {
			b, err := io.ReadAll(f)
			if err != nil {
				logrus.Fatalln(err)
			}
			if padding.UpdatePaddingScheme(b) {
				logrus.Infoln("loaded padding scheme file:", *paddingScheme)
			} else {
				logrus.Errorln("wrong format padding scheme file:", *paddingScheme)
			}
			f.Close()
		} else {
			logrus.Fatalln(err)
		}
	}

	logLevel, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)

	var sum = sha256.Sum256([]byte(*password))
	passwordSha256 = sum[:]

	logrus.Infoln("[Server]", util.ProgramVersionName)
	logrus.Infof("[Server] Version: %s, Build: %s, Commit: %s", Version, BuildTime, GitCommit)
	logrus.Infoln("[Server] Listening TCP", *listen)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to listen on TCP:", *listen)
	}

	tlsCert, _ := util.GenerateKeyPair(time.Now, *sni)
	tlsConfig := &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return tlsCert, nil
		},
	}

	ctx := context.Background()
	server := NewMyServer(tlsConfig, *dial, *dialFallback, *healthCheckURLs, *healthCheckInterval, *healthCheckTimeout, *healthCheckThreshold, *dataTransferIdle, *connectTimeout, *readTimeout, *writeTimeout)

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 监听信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动连接处理循环
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return // 服务器正在关闭
				default:
					logrus.WithError(err).Errorln("Failed to accept connection")
				}
				continue
			}

			atomic.AddInt64(&connectionCount, 1)
			go func(conn net.Conn) {
				defer func() {
					conn.Close()
					atomic.AddInt64(&connectionCount, -1)
				}()
				handleTcpConnection(ctx, conn, server)
			}(c)
		}
	}()

	// 定期输出连接统计
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count := atomic.LoadInt64(&connectionCount)
				if count > 0 {
					logrus.Infof("[Server] Active connections: %d", count)
				}
			}
		}
	}()

	// 等待关闭信号
	<-sigChan
	logrus.Infoln("[Server] Shutting down gracefully...")

	// 取消上下文，停止接受新连接
	cancel()

	// 等待所有连接完成（最多等待30秒）
	shutdownTimeout := time.NewTimer(30 * time.Second)
	defer shutdownTimeout.Stop()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownTimeout.C:
			logrus.Warnln("[Server] Shutdown timeout, forcing exit")
			os.Exit(1)
		case <-ticker.C:
			count := atomic.LoadInt64(&connectionCount)
			if count == 0 {
				logrus.Infoln("[Server] All connections closed, exiting")
				return
			}
			logrus.Infof("[Server] Waiting for %d connections to close...", count)
		}
	}
}
