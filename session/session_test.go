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
  getConns := func() (*net.TCPConn, *net.TCPConn) {
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
    return conn1, conn2
  }
  conn1, conn2 := getConns()

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
  session2 := ev.Session

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

  if session2.maxReceivedSerial < uint32(n) {
    t.Fatal("serial not match")
  }

  <-time.After(time.Millisecond * 1000)
  if session1.maxAckSerial == 0 {
    t.Fatal("no ack received")
  }

  if comm1.BytesSent != comm2.BytesReceived {
    t.Fatal("bytes sent not equal to bytes received")
  }

  comm1.Close()
  comm2.Close()
  if !comm1.IsClosed { t.Fatal("comm not closed") }
  if !comm2.IsClosed { t.Fatal("comm not closed") }

  // test connection reset
  //conn1, conn2 = getConns()
  //comm1 = NewComm(conn1, key, nil)
  //comm2 = NewComm(conn2, key, nil)
  //n = 10240
  //go func() {
  //  x := 0
  //  for {
  //    ev := <-comm2.Events
  //    if ev.Type != DATA { continue }
  //    s := string(ev.Data)
  //    expected := fmt.Sprintf("data-%d", x)
  //    if string(ev.Data) != fmt.Sprintf("data-%d", x) {
  //      t.Fatal("expected ", expected, " get ", s)
  //    }
  //    x += 1
  //  }
  //}()
  //session1 = comm1.NewSession(-1, nil, nil)
  //for i := 0; i < n; i++ {
  //  session1.Send([]byte(fmt.Sprintf("data-%d", i)))
  //  if i % 128 == 0 {
  //    conn1, conn2 = getConns()
  //    comm1 = NewComm(conn1, key, comm1)
  //    comm2 = NewComm(conn2, key, comm2)
  //    fmt.Printf("connection reset at %d, %v %v\n", i, conn1.RemoteAddr(), conn2.LocalAddr())
  //  }
  //}
}
