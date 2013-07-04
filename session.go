package main

import (
  "net"
  "io"
  "encoding/binary"
  cr "./conn_reader"
  "errors"
)

type Session struct {
  Id int64
  Conn *net.TCPConn
  HostPort string
  ConnReader *cr.ConnReader
}

const (
  CONNECT = uint8(0)
  DATA = uint8(1)
  DISCONNECT = uint8(2)
)

func ReadHeader(reader *net.TCPConn) (int64, uint8, error) {
  var id int64
  var t uint8
  var err error
  err = binary.Read(reader, binary.BigEndian, &id)
  if err != nil { return 0, 0, err }
  err = binary.Read(reader, binary.BigEndian, &t)
  if err != nil { return 0, 0, err }
  return id, t, nil
}

func (self *Session) WriteHeader(t uint8) (err error) {
  err = binary.Write(self.Conn, binary.BigEndian, self.Id)
  if err != nil { return }
  err = binary.Write(self.Conn, binary.BigEndian, t)
  return
}

func (self *Session) Connect() (err error) {
  err = self.WriteHeader(CONNECT)
  if err != nil { return }
  hostPortLen := uint8(len(self.HostPort))
  err = binary.Write(self.Conn, binary.BigEndian, hostPortLen)
  if err != nil { return }
  n, err := self.Conn.Write([]byte(self.HostPort))
  if err != nil { return }
  if uint8(n) != hostPortLen { return errors.New("") }
  return
}

func (self *Session) Accept() (err error) {
  var hostPortLen uint8
  err = binary.Read(self.Conn, binary.BigEndian, &hostPortLen)
  if err != nil { return }
  hostPort := make([]byte, hostPortLen)
  n, err := io.ReadFull(self.Conn, hostPort)
  if err != nil { return }
  if uint8(n) != hostPortLen { return errors.New("") }
  self.HostPort = string(hostPort[:hostPortLen])
  //TODO connect to remote host, then add conn to conn reader
  return
}

/*
send command
read command
commands: CONNECT DATA DISCONNECT
send connect
receive connect
send data
receive data
send disconnect
receive disconnect
*/
