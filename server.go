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
  "key": "foo bar baz foo bar baz ",
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
  checkConfig("key")
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

func handleClient(conn *net.TCPConn) {
  defer conn.Close()

  t1 := time.Now()
  delta := func() string {
    return fmt.Sprintf("%-15v", time.Now().Sub(t1))
  }

  targetReader := cr.New()
  comm := session.NewComm(conn, []byte(globalConfig["key"]))
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
    return
  }

  loop: for { select {
  // local-side events
  case ev := <-comm.Events:
    switch ev.Type {
    case session.SESSION: // new local session
      hostPort := string(ev.Data)
      if hostPort == keepaliveSessionMagic { continue loop }
      serv := &Serv{
        sendQueue: make(chan []byte, 65536),
        hostPort: hostPort,
      }
      serv.session = ev.Session
      ev.Session.Obj = serv
      go connectTarget(serv)
    case session.DATA: // local data
      serv := ev.Session.Obj.(*Serv)
      if serv.targetConn == nil {
        serv.sendQueue <- ev.Data
      } else {
        serv.targetConn.Write(ev.Data)
      }
    case session.SIGNAL: // local session closed
      sig := ev.Data[0]
      if sig == sigClose {
        serv := ev.Session.Obj.(*Serv)
        if serv.targetConn != nil {
          go func() {
            <-time.After(time.Second * 5)
            serv.targetConn.Close()
          }()
        }
        serv.remoteClosed = true
        if serv.localClosed { serv.session.Close() }
      } else if sig == sigPing {
        fmt.Printf("%s pong\n", delta())
        ev.Session.Signal(sigPing)
      }
    case session.ERROR: // error
      break loop
    }
  // target connection events
  case serv := <-targetConnEvents:
    readQueue: for { select {
    case data := <-serv.sendQueue:
      serv.targetConn.Write(data)
    default: break readQueue
    }}
  // target events
  case ev := <-targetReader.Events:
    serv := ev.Obj.(*Serv)
    switch ev.Type {
    case cr.DATA:
      serv.session.Send(ev.Data)
    case cr.EOF, cr.ERROR:
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed { serv.session.Close() }
    }
  goto loop
  }}
}
