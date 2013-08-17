package socks

import (
  "net"
  "errors"
  "fmt"
  "strings"
  "encoding/binary"
  "bytes"
  "strconv"
  "../utils"
)

type Server struct {
  ln *net.TCPListener
  isStopped bool
  Clients <-chan *Client
  ClientsIn chan *Client
}

func (self *Server) Close() {
  self.isStopped = true
  self.ln.Close()
}

func New(listenAddr string) (*Server, error) {
  server := &Server{
    ClientsIn: make(chan *Client),
  }
  server.Clients = utils.MakeChan(server.ClientsIn).(<-chan *Client)
  addr, err := net.ResolveTCPAddr("tcp", listenAddr)
  if err != nil { return nil, server.newError(err) }
  ln, err := net.ListenTCP("tcp", addr)
  if err != nil { return nil, server.newError(err) }
  server.ln = ln
  go func() {
    for {
      conn, err := ln.AcceptTCP()
      if err != nil {
        if server.isStopped { return }
        continue
      }
      go func() {
        err := server.handshake(conn)
        if err != nil {
          conn.Close()
          return
        }
      }()
    }
  }()
  return server, nil
}

func (self *Server) newError(args ...interface{}) error {
  msg := make([]string, 0)
  for _, arg := range args {
    msg = append(msg, fmt.Sprintf("%v", arg))
  }
  return errors.New("<Socket Server> " + strings.Join(msg, " "))
}

func (self *Server) handshake(conn *net.TCPConn) error {
  var ver, nMethods byte

  // handshake
  err := binary.Read(conn, binary.BigEndian, &ver)
  if err != nil { return self.newError("handshake", err) }
  err = binary.Read(conn, binary.BigEndian, &nMethods)
  if err != nil { return self.newError("handshake", err) }
  methods := make([]byte, nMethods)
  err = binary.Read(conn, binary.BigEndian, methods)
  if err != nil { return self.newError("handshake", err) }
  err = binary.Write(conn, binary.BigEndian, VERSION)
  if err != nil { return self.newError("handshake", err) }
  if ver != VERSION || nMethods < byte(1) {
    err = binary.Write(conn, binary.BigEndian, METHOD_NO_ACCEPTABLE)
    if err != nil { return self.newError("handshake", err) }
  } else {
    if bytes.IndexByte(methods, METHOD_NOT_REQUIRED) == -1 {
      err = binary.Write(conn, binary.BigEndian, METHOD_NO_ACCEPTABLE)
      if err != nil { return self.newError("handshake", err) }
    } else {
      err = binary.Write(conn, binary.BigEndian, METHOD_NOT_REQUIRED)
      if err != nil { return self.newError("handshake", err) }
    }
  }

  // request
  var cmd, reserved, addrType byte
  err = binary.Read(conn, binary.BigEndian, &ver)
  if err != nil { return self.newError("handshake", err) }
  err = binary.Read(conn, binary.BigEndian, &cmd)
  if err != nil { return self.newError("handshake", err) }
  err = binary.Read(conn, binary.BigEndian, &reserved)
  if err != nil { return self.newError("handshake", err) }
  err = binary.Read(conn, binary.BigEndian, &addrType)
  if err != nil { return self.newError("handshake", err) }
  if ver != VERSION { return self.newError("handshake") }
  if reserved != RESERVED { return self.newError("handshake") }
  if addrType != ADDR_TYPE_IP && addrType != ADDR_TYPE_DOMAIN && addrType != ADDR_TYPE_IPV6 {
    writeAck(conn, REP_ADDRESS_TYPE_NOT_SUPPORTED)
    return self.newError("handshake")
  }

  var address []byte
  if addrType == ADDR_TYPE_IP {
    address = make([]byte, 4)
  } else if addrType == ADDR_TYPE_DOMAIN {
    var domainLength byte
    err := binary.Read(conn, binary.BigEndian, &domainLength)
    if err != nil { return self.newError("handshake", err) }
    address = make([]byte, domainLength)
  } else if addrType == ADDR_TYPE_IPV6 {
    address = make([]byte, 16)
  }
  err = binary.Read(conn, binary.BigEndian, address)
  if err != nil { return self.newError("handshake", err) }
  var port uint16
  err = binary.Read(conn, binary.BigEndian, &port)
  if err != nil { return self.newError("handshake", err) }

  var hostPort string
  if addrType == ADDR_TYPE_IP || addrType == ADDR_TYPE_IPV6 {
    ip := net.IP(address)
    hostPort = net.JoinHostPort(ip.String(), strconv.Itoa(int(port)))
  } else if addrType == ADDR_TYPE_DOMAIN {
    hostPort = net.JoinHostPort(string(address), strconv.Itoa(int(port)))
  }

  if cmd != CMD_CONNECT {
    writeAck(conn, REP_COMMAND_NOT_SUPPORTED)
    return self.newError("handshake")
  }
  writeAck(conn, REP_SUCCEED)

  client := &Client{
    Conn: conn,
    HostPort: hostPort,
  }
  self.ClientsIn <- client

  return nil
}

func writeAck(conn *net.TCPConn, reply byte) error {
  err := binary.Write(conn, binary.BigEndian, VERSION)
  if err != nil { return errors.New("") }
  err = binary.Write(conn, binary.BigEndian, reply)
  if err != nil { return errors.New("") }
  err = binary.Write(conn, binary.BigEndian, RESERVED)
  if err != nil { return errors.New("") }
  err = binary.Write(conn, binary.BigEndian, ADDR_TYPE_IP)
  if err != nil { return errors.New("") }
  err = binary.Write(conn, binary.BigEndian, [4]byte{0, 0, 0, 0})
  if err != nil { return errors.New("") }
  err = binary.Write(conn, binary.BigEndian, uint16(0))
  if err != nil { return errors.New("") }
  return nil
}
