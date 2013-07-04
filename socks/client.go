package socks

import (
  "net"
)

type Client struct {
  Conn *net.TCPConn
  HostPort string
}
