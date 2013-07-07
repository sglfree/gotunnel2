package socks

import (
  "testing"
  "fmt"
  "time"
)

func TestSocks(t *testing.T) {
  server, err := New("localhost:54321")
  if err != nil {
    t.Fatal(err)
  }
  defer server.Close()
  for {
    select {
    case client := <-server.Clients:
      fmt.Printf("%s\n", client)
    case <-time.After(time.Second * 1):
      return
    }
  }
}
