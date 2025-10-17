package proxy

import (
	"net"
	"time"
)

// SystemDialer 系统拨号器，用于客户端直连服务端
var SystemDialer = &net.Dialer{
	Timeout: time.Second * 30,
}
