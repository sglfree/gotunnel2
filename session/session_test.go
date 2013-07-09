package session

import (
  "testing"
  "net"
  "time"
  "fmt"
  "bytes"
)

func TestSession(t *testing.T) {
  addr1, err := net.ResolveTCPAddr("tcp", "localhost:42222")
  if err != nil { t.Fatal(err) }
  ln, err := net.ListenTCP("tcp", addr1)
  if err != nil { t.Fatal(err) }
  var conn1 *net.TCPConn
  ready := make(chan struct{})
  go func() {
    var err error
    conn1, err = ln.AcceptTCP()
    if err != nil { t.Fatal(err) }
    close(ready)
  }()

  conn2, err := net.DialTCP("tcp", nil, addr1)
  if err != nil { t.Fatal(err) }
  <-ready

  key := bytes.Repeat([]byte("foo bar "), 3)
  comm1 := NewComm(conn1, key)
  comm2 := NewComm(conn2, key)

  greeting := []byte("hello")
  session1 := comm1.NewSession(0, greeting, nil)
  id1 := session1.Id

  var ev Event
  select {
  case ev = <-comm2.Events:
  case <-time.After(time.Second * 1):
    t.Fatal("event timeout")
  }
  if ev.Type != SESSION || ev.Session.Id != id1 || bytes.Compare(ev.Data, greeting) != 0 {
    t.Fatal("event not match")
  }

  n := 2048
  for i := 0; i < n; i++ {
    s := bytes.Repeat([]byte(fmt.Sprintf("Hello, %d world!", i)), i)
    session1.Send(s)
    select {
    case ev = <-comm2.Events:
    case <-time.After(time.Second * 1):
      t.Fatal("event timeout again")
    }
    if ev.Type != DATA || ev.Session.Id != id1 {
      t.Fatal("event not match again")
    }
    if bytes.Compare(s, ev.Data) != 0 {
      t.Fatal(fmt.Sprintf("%d data not match: %x", i, ev.Data))
    }
  }

  sigClose := uint8(0)
  session1.Signal(sigClose)
  select {
  case ev = <-comm2.Events:
  case <-time.After(time.Second * 1):
    t.Fatal("event timeout here")
  }
  if ev.Type != SIGNAL || ev.Session.Id != id1 || ev.Data[0] != sigClose {
    t.Fatal("event not match here")
  }

  if comm2.maxReceivedSerial < uint64(n) {
    t.Fatal("serial not match")
  }

  <-time.After(time.Millisecond * 500)
  fmt.Printf("max ack %d\n", comm1.maxAckSerial)
  if comm1.maxAckSerial == 0 {
    t.Fatal("no ack received")
  }
}
