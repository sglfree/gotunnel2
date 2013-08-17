package conn_reader

import (
  "net"
  "math/rand"
  "time"
  "io"
  "sync/atomic"
  "../utils"
)

const BUFFER_SIZE = 1280

func init() {
  rand.Seed(time.Now().UnixNano())
}

type ConnReader struct {
  Events <-chan Event
  EventsIn chan Event
  Count int32
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
  self.EventsIn = make(chan Event)
  self.Events = utils.MakeChan(self.EventsIn).(<-chan Event)
  return self
}

func (self *ConnReader) Add(tcpConn *net.TCPConn, obj interface{}) {
  atomic.AddInt32(&self.Count, int32(1))
  go func() {
    defer atomic.AddInt32(&self.Count, int32(-1))
    for {
      buf := make([]byte, BUFFER_SIZE)
      n, err := tcpConn.Read(buf)
      if n > 0 {
        self.EventsIn <- Event{DATA, buf[:n], obj}
      }
      if err != nil {
        if err == io.EOF { //EOF
          self.EventsIn <- Event{EOF, nil, obj}
        } else { //ERROR
          self.EventsIn <- Event{ERROR, []byte(err.Error()), obj}
        }
        return
      }
    }
  }()
}
