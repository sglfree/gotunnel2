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

  var msg Message
  select {
  case msgI := <-comm2.Messages.Out:
    msg = msgI.(Message)
  case <-time.After(time.Second * 1):
    t.Fatal("message timeout")
  }
  if msg.Type != SESSION || msg.Session.Id != id1 || bytes.Compare(msg.Data, greeting) != 0 {
    t.Fatal("message not match")
  }

  for i := 0; i < 1024; i++ {
    s := []byte(fmt.Sprintf("Hello, %d world!", i))
    session1.Send(s)
    select {
    case msgI := <-comm2.Messages.Out:
      msg = msgI.(Message)
    case <-time.After(time.Second * 1):
      t.Fatal("message timeout again")
    }
    if msg.Type != DATA || msg.Session.Id != id1 {
      t.Fatal("message not match again")
    }
    if bytes.Compare(s, msg.Data) != 0 {
      t.Fatal("data not match")
    }
  }

  session1.Close()
  select {
  case msgI := <-comm2.Messages.Out:
    msg = msgI.(Message)
  case <-time.After(time.Second * 1):
    t.Fatal("message timeout here")
  }
  if msg.Type != CLOSE || msg.Session.Id != id1 {
    t.Fatal("message not match here")
  }
}
