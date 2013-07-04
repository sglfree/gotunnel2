package conn_reader

import (
  "net"
  ic "../infinite_chan"
  "math/rand"
  "time"
  "io"
)

const BUFFER_SIZE = 2048

func init() {
  rand.Seed(time.Now().UnixNano())
}

type ConnReader struct {
  Messages *ic.InfiniteChan
}

type Info struct {
  TCPConn *net.TCPConn
  Id int64
}

const (
  DATA = iota
  EOF
  ERROR
)

type Message struct {
  Info *Info
  Type int
  Data []byte
}

func New() *ConnReader {
  self := new(ConnReader)
  self.Messages = ic.New()
  return self
}

func (self *ConnReader) Close() {
  self.Messages.Close()
}

func (self *ConnReader) Add(tcpConn *net.TCPConn) *Info {
  info := &Info{
    TCPConn: tcpConn,
    Id: rand.Int63(),
  }
  go func() {
    for {
      buf := make([]byte, 2048)
      n, err := tcpConn.Read(buf)
      if err != nil {
        if err == io.EOF { //EOF
          self.Messages.In <- Message{info, EOF, nil}
        } else { //ERROR
          self.Messages.In <- Message{info, ERROR, []byte(err.Error())}
        }
        return
      }
      self.Messages.In <- Message{info, DATA, buf[:n]}
    }
  }()
  return info
}
