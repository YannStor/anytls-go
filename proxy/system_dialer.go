package proxy

import (
	"net"
	"time"
)

// SystemDialer 系统拨号器，用于客户端直连服务端
var SystemDialer = &net.Dialer{
	Timeout: time.Second * 30,
}
```

现在客户端应该可以正常工作了。这个 `SystemDialer` 只是客户端用来直连服务端的，不涉及任何代理功能。代理功能完全在服务端实现。
