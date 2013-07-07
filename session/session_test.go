package session

import (
  "testing"
  "net"
  "bytes"
  "time"
  "fmt"
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

  comm1 := NewComm(conn1)
  comm2 := NewComm(conn2)

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

  for i := 0; i < 1024; i++ {
    s := []byte(fmt.Sprintf("Hello, %d world!", i))
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
      t.Fatal("data not match")
    }
  }

  session1.Close()
  select {
  case ev = <-comm2.Events:
  case <-time.After(time.Second * 1):
    t.Fatal("event timeout here")
  }
  if ev.Type != CLOSE || ev.Session.Id != id1 {
    t.Fatal("event not match here")
  }
}
