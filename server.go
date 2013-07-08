package main

import (
  "net"
  "log"
  "fmt"
  cr "./conn_reader"
  "./session"
  "time"
)

// configuration
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

type Serv struct {
  session *session.Session
  sendQueue chan []byte
  targetConn *net.TCPConn
  hostPort string
  localClosed bool
  remoteClosed bool
}

const sigClose = uint8(0)

func handleClient(conn *net.TCPConn) {
  defer conn.Close()

  targetReader := cr.New()
  comm := session.NewComm(conn)
  targetConnEvents := make(chan *Serv, 65536)
  connectTarget := func(serv *Serv) {
    defer func() {
      targetConnEvents <- serv
    }()
    addr, err := net.ResolveTCPAddr("tcp", serv.hostPort)
    if err != nil { return }
    targetConn, err := net.DialTCP("tcp", nil, addr)
    if err != nil { return }
    targetReader.Add(targetConn, serv)
    serv.targetConn = targetConn
    fmt.Printf("target connected %s\n", serv.hostPort)
    return
  }

  loop: for { select {
  // local-side events
  case ev := <-comm.Events:
    switch ev.Type {
    case session.SESSION: // new local session
      serv := &Serv{
        sendQueue: make(chan []byte, 65536),
        hostPort: string(ev.Data),
      }
      fmt.Printf("new session %s\n", serv.hostPort)
      serv.session = ev.Session
      ev.Session.Obj = serv
      go connectTarget(serv)
    case session.DATA: // local data
      serv := ev.Session.Obj.(*Serv)
      fmt.Printf("%d data, %s\n", len(ev.Data), serv.hostPort)
      if serv.targetConn == nil {
        serv.sendQueue <- ev.Data
        fmt.Printf("enqueue\n")
      } else {
        serv.targetConn.Write(ev.Data)
        fmt.Printf("sent\n")
      }
    case session.SIGNAL: // local session closed
      if ev.Data[0] == sigClose {
        serv := ev.Session.Obj.(*Serv)
        if serv.targetConn != nil {
          go func() {
            <-time.After(time.Second * 5)
            serv.targetConn.Close()
          }()
        }
        serv.remoteClosed = true
        if serv.localClosed { serv.session.Close() }
      }
    case session.ERROR: // error
      break loop
    }
  // target connection events
  case serv := <-targetConnEvents:
    fmt.Printf("target ready, %s\n", serv.hostPort)
    readQueue: for { select {
    case data := <-serv.sendQueue:
      serv.targetConn.Write(data)
      fmt.Printf("sent %d bytes\n", len(data))
    default: break readQueue
    }}
    fmt.Printf("queue sent, %s\n", serv.hostPort)
  // target events
  case ev := <-targetReader.Events:
    serv := ev.Obj.(*Serv)
    switch ev.Type {
    case cr.DATA:
      fmt.Printf("receive %d bytes from %s\n", len(ev.Data), serv.hostPort)
      serv.session.Send(ev.Data)
    case cr.EOF, cr.ERROR:
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed { serv.session.Close() }
    }
  goto loop
  }}
}
