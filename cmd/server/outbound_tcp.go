package main

import (
	"anytls/proxy/simpledialer"
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sirupsen/logrus"
)

func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
	logrus.Debugf("ProxyOutboundTCP: New connection from %s to %s", conn.RemoteAddr(), destination)

	// 获取拨号器
	dialerInterface := server.GetDialer()

	var outboundConn net.Conn
	var err error

	if proxyDialer, ok := dialerInterface.(*simpledialer.SimpleDialer); ok {
		// 使用简化代理拨号器
		outboundConn, err = proxyDialer.DialContext(ctx, "tcp", destination.String())
		if err != nil {
			logrus.Debugln("TCP proxy failed:", err)
			return E.Errors(err, N.ReportHandshakeFailure(conn, err))
		}
		logrus.Debugln("Using TCP proxy for:", destination.String(), "via", proxyDialer.GetCurrentProxy())
	} else if dialer, ok := dialerInterface.(net.Dialer); ok {
		// 使用系统拨号器
		outboundConn, err = dialer.DialContext(ctx, "tcp", destination.String())
		if err != nil {
			logrus.Debugln("Direct dial failed:", err)
			return E.Errors(err, N.ReportHandshakeFailure(conn, err))
		}
		logrus.Debugln("Using direct TCP for:", destination.String())
	} else {
		// 回退到默认拨号
		outboundConn, err = net.DialTCP("tcp", nil, &net.TCPAddr{
			IP:   net.ParseIP(destination.Addr.String()),
			Port: int(destination.Port),
		})
		if err != nil {
			logrus.Debugln("Default dial failed:", err)
			return E.Errors(err, N.ReportHandshakeFailure(conn, err))
		}
		logrus.Debugln("Using default TCP for:", destination.String())
	}

	// 报告握手成功
	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		outboundConn.Close()
		return err
	}

	// 开始数据转发
	defer outboundConn.Close()
	return bufio.CopyConn(ctx, conn, outboundConn)
}

func proxyOutboundUoT(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
	request, err := uot.ReadRequest(conn)
	if err != nil {
		logrus.Debugln("proxyOutboundUoT ReadRequest:", err)
		return err
	}

	// 尝试使用代理拨号器
	dialerInterface := server.GetDialer()
	var c net.PacketConn

	if proxyDialer, ok := dialerInterface.(*simpledialer.SimpleDialer); ok {
		// 尝试通过简化代理拨号器创建 UDP 连接
		udpConn, err := proxyDialer.DialContext(ctx, "tcp", destination.String()) // 简化拨号器只支持 TCP
		if err == nil {
			// 对于 UDP over TCP，我们使用 TCP 连接
			c = &ConnToPacketConnAdapter{Conn: udpConn}
			logrus.Debugln("Using TCP proxy for UoT:", destination.String(), "via", proxyDialer.GetCurrentProxy())
		} else {
			logrus.Debugln("TCP proxy failed for UoT, using local UDP:", err)
			// 代理失败，回退到本地 UDP
			c, err = net.ListenPacket("udp", "")
			if err != nil {
				logrus.Debugln("proxyOutboundUoT ListenPacket:", err)
				err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
				return err
			}
		}
	} else {
		// 没有代理拨号器，使用本地 UDP
		c, err = net.ListenPacket("udp", "")
		if err != nil {
			logrus.Debugln("proxyOutboundUoT ListenPacket:", err)
			err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
			return err
		}
	}

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		c.Close()
		return err
	}

	return bufio.CopyPacketConn(ctx, uot.NewConn(conn, *request), bufio.NewPacketConn(c))
}

// ConnToPacketConnAdapter 将 net.Conn 适配为 net.PacketConn
type ConnToPacketConnAdapter struct {
	net.Conn
}

// WriteTo 实现 PacketConn 的 WriteTo 方法
func (a *ConnToPacketConnAdapter) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return a.Conn.Write(p)
}

// ReadFrom 实现 PacketConn 的 ReadFrom 方法
func (a *ConnToPacketConnAdapter) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = a.Conn.Read(p)
	addr = a.Conn.RemoteAddr()
	return
}
