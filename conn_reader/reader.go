package conn_reader

import (
  "net"
  ic "../infinite_chan"
  "math/rand"
  "time"
  "io"
)

const BUFFER_SIZE = 11200

func init() {
  rand.Seed(time.Now().UnixNano())
}

type ConnReader struct {
  Events *ic.InfiniteChan
}

const (
  DATA = iota
  EOF
  ERROR
)

type Event struct {
  Type int
  Data []byte
  Obj interface{}
}

func New() *ConnReader {
  self := new(ConnReader)
  self.Events = ic.New()
  return self
}

func (self *ConnReader) Close() {
  self.Events.Close()
}

func (self *ConnReader) Add(tcpConn *net.TCPConn, obj interface{}) {
  go func() {
    for {
      buf := make([]byte, BUFFER_SIZE)
      n, err := tcpConn.Read(buf)
      if n > 0 {
        self.Events.In <- Event{DATA, buf[:n], obj}
      }
      if err != nil {
        if err == io.EOF { //EOF
          self.Events.In <- Event{EOF, nil, obj}
        } else { //ERROR
          self.Events.In <- Event{ERROR, []byte(err.Error()), obj}
        }
        return
      }
    }
  }()
}
