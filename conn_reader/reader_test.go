package conn_reader

import (
  "testing"
  "net"
  "fmt"
)

func TestConnReader(t *testing.T) {
  reader := New()
  addr, _ := net.ResolveTCPAddr("tcp", "localhost:54322")
  ready := make(chan struct{})
  infos := make(map[int64]int)
  go func() {
    ln, err := net.ListenTCP("tcp", addr)
    if err != nil { t.Fatal(err) }
    close(ready)
    for {
      conn, err := ln.AcceptTCP()
      if err != nil { t.Fatal(err) }
      info := reader.Add(conn)
      infos[info.Id] = 0
    }
  }()
  <-ready
  n := 50
  for i := 0; i < n; i++ {
    conn, err := net.DialTCP("tcp", nil, addr)
    if err != nil { t.Fatal(err) }
    conn.Write([]byte(fmt.Sprintf("%d", i)))
    conn.Close()
  }
  received := 0
  for {
    msg := (<-reader.Messages.Out).(Message)
    switch msg.Type {
    case DATA:
      fmt.Printf("%d %s\n", msg.Info.Id, msg.Data)
      infos[msg.Info.Id] += 1
    case EOF:
      msg.Info.TCPConn.Close()
      received += 1
    case ERROR:
      t.Fatal(msg.Data)
    }
    if received == n {
      break
    }
  }
}
