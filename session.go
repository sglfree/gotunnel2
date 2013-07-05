package main

import (
  "net"
  "io"
  "encoding/binary"
  "errors"
  "fmt"
  "time"
  ic "./infinite_chan"
)

var startTime = time.Now()

type Session struct {
  Id int64
  RemoteConn *net.TCPConn
  LocalConn *net.TCPConn
  HostPort string
  remoteClosed bool
  localClosed bool
  SendQueue *ic.InfiniteChan
}

const (
  CONNECT = uint8(0)
  DATA = uint8(1)
  EOF = uint8(2)
  ERROR = uint8(3)
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
  err = binary.Write(self.RemoteConn, binary.BigEndian, self.Id)
  if err != nil { return }
  err = binary.Write(self.RemoteConn, binary.BigEndian, t)
  return
}

func (self *Session) Connect() (err error) {
  err = self.WriteHeader(CONNECT)
  if err != nil { return }
  hostPortLen := uint8(len(self.HostPort))
  err = binary.Write(self.RemoteConn, binary.BigEndian, hostPortLen)
  if err != nil { return }
  n, err := self.RemoteConn.Write([]byte(self.HostPort))
  if err != nil { return }
  if uint8(n) != hostPortLen { return errors.New("") }
  return
}

func (self *Session) Accept() (err error) {
  var hostPortLen uint8
  err = binary.Read(self.RemoteConn, binary.BigEndian, &hostPortLen)
  if err != nil { return }
  hostPort := make([]byte, hostPortLen)
  n, err := io.ReadFull(self.RemoteConn, hostPort)
  if err != nil { return }
  if uint8(n) != hostPortLen { return errors.New("") }
  self.HostPort = string(hostPort[:hostPortLen])
  return
}

func (self *Session) Write(data []byte) (err error) {
  err = self.WriteHeader(DATA)
  if err != nil { return }
  dataLen := uint32(len(data))
  err = binary.Write(self.RemoteConn, binary.BigEndian, dataLen)
  if err != nil { return }
  n, err := self.RemoteConn.Write(data)
  if err != nil { return }
  if uint32(n) != dataLen { return errors.New("") }
  return
}

func (self *Session) Read() (data []byte, err error) {
  var dataLen uint32
  err = binary.Read(self.RemoteConn, binary.BigEndian, &dataLen)
  if err != nil { return }
  data = make([]byte, dataLen)
  n, err := io.ReadFull(self.RemoteConn, data)
  if err != nil { return }
  if uint32(n) != dataLen { return nil, errors.New("") }
  return
}

func (self *Session) log(args ...interface{}) {
  fmt.Printf("%12.6f <%020d> ", float64(time.Now().UnixNano()) / float64(1000000000), self.Id)
  for _, arg := range args {
    fmt.Printf(" %v", arg)
  }
  fmt.Printf("\n")
}

type Message struct {
  Type uint8
  Session *Session
  Data []byte
}

func startRemoteReader(sessions map[int64]*Session, conn *net.TCPConn) *ic.InfiniteChan {
  c := ic.New()
  go func() {
    loop: for {
      id, t, err := ReadHeader(conn)
      if err != nil {
        c.Out <- Message{ERROR, nil, nil}
      }
      switch t {
      case CONNECT:
        session := &Session{
          Id: id,
          RemoteConn: conn,
        }
        err = session.Accept()
        if err != nil {
          session.log("error when accept connection", err)
          continue loop
        }
        sessions[session.Id] = session
        session.log("created", session.HostPort)
        c.In <- Message{CONNECT, session, nil}
      case DATA:
        session, ok := sessions[id]
        if !ok { continue loop }
        data, err := session.Read()
        if err != nil {
          c.In <- Message{ERROR, nil, nil}
          return
        }
        c.In <- Message{DATA, session, data}
      case EOF:
        session, ok := sessions[id]
        if !ok { continue loop }
        session.LocalConn.Close()
        session.log("remote >> EOF")
        session.remoteClosed = true
        if session.localClosed {
          if session.SendQueue != nil {
            session.SendQueue.Close()
          }
          delete(sessions, session.Id)
          session.log("cleared")
        }
      }
    }
  }()
  return c
}
