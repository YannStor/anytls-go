package main

import (
	"anytls/proxy/dialer"
	"context"
	"fmt"
	"net"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
	"github.com/sirupsen/logrus"
)

func proxyOutboundTCP(ctx context.Context, conn net.Conn, destination M.Socksaddr, server *myServer) error {
	dialerInterface := server.GetDialer()
	var c net.Conn
	var err error

	// 尝试转换为 ContextDialer
	if contextDialer, ok := dialerInterface.(interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}); ok {
		c, err = contextDialer.DialContext(ctx, "tcp", destination.String())
	} else if dialer, ok := dialerInterface.(net.Dialer); ok {
		c, err = dialer.Dial("tcp", destination.String())
	} else {
		err = fmt.Errorf("unsupported dialer type")
	}
	if err != nil {
		logrus.Debugln("proxyOutboundTCP DialContext:", err)
		err = E.Errors(err, N.ReportHandshakeFailure(conn, err))
		return err
	}

	err = N.ReportHandshakeSuccess(conn)
	if err != nil {
		return err
	}

	return bufio.CopyConn(ctx, conn, c)
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

	if proxyDialer, ok := dialerInterface.(*dialer.ProxyDialer); ok {
		// 尝试通过代理创建 UDP 连接
		udpConn, err := proxyDialer.DialContext(ctx, "udp", destination.String())
		if err == nil {
			// 代理成功，将 net.Conn 转换为 PacketConnnn
			if packetConn, ok := udpConn.(net.PacketConn); ok {
				c = packetConn
			} else {
				// 如果不是 PacketConn，创建适配器
				c = &ConnToPacketConnAdapter{Conn: udpConn}
			}
			logrus.Debugln("Using UDP proxy for UoT:", destination.String())
		} else {
			logrus.Debugln("UDP proxy failed, using local UDP:", err)
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
