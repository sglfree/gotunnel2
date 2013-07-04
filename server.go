package main

import (
  "net"
  "log"
  "fmt"
)

var defaultConfig = map[string]string{
  "listen": "0.0.0.0:34567",
}
var globalConfig = loadConfig(defaultConfig)
func checkConfig(key string) {
  if value, ok := globalConfig[key]; !ok || value == "" {
    globalConfig[key] = defaultConfig[key]
    saveConfig(globalConfig)
    globalConfig = loadConfig(defaultConfig)
  }
}

func init() {
  checkConfig("listen")
}

func main() {
  addr, err := net.ResolveTCPAddr("tcp", globalConfig["listen"])
  if err != nil { log.Fatal("cannot resolve listen address ", err) }
  ln, err := net.ListenTCP("tcp", addr)
  if err != nil { log.Fatal("cannot listen ", err) }
  fmt.Printf("server listening on %v\n", ln.Addr())
  for {
    conn, err := ln.AcceptTCP()
    if err != nil { continue }
    go handleClient(conn)
  }
}

func handleClient(conn *net.TCPConn) {
  defer conn.Close()
  for {
    id, t, err := ReadHeader(conn)
    if err != nil { return }
    switch t {
    case CONNECT:
      session := &Session{
        Id: id,
        Conn: conn,
        //TODO ConnReader
      }
      err := session.Accept()
      if err != nil { log.Fatal(err) }
      fmt.Printf("%s\n", session.HostPort)
    }
  }
}
