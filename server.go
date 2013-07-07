package main

import (
  "net"
  "log"
  "fmt"
  cr "./conn_reader"
  ic "./infinite_chan"
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

type Session struct {
  session *session.Session
  sendQueue *ic.InfiniteChan
  targetConn *net.TCPConn
  hostPort string
}

func handleClient(conn *net.TCPConn) {
  defer conn.Close()

  targetReader := cr.New()
  comm := session.NewComm(conn)
  targetConnEvents := ic.New()
  connectTarget := func(session *Session) {
    defer func() {
      targetConnEvents.In <- session
    }()
    addr, err := net.ResolveTCPAddr("tcp", session.hostPort)
    if err != nil { return }
    targetConn, err := net.DialTCP("tcp", nil, addr)
    if err != nil { return }
    targetReader.Add(targetConn, session)
    session.targetConn = targetConn
    return
  }

  loop: for { select {
  // local-side events
  case evI := <-comm.Events.Out:
    ev := evI.(session.Event)
    switch ev.Type {
    case session.SESSION: // new local session
      session := &Session{
        sendQueue: ic.New(),
        hostPort: string(ev.Data),
      }
      session.session = comm.NewSession(ev.Session.Id, nil, session)
      go connectTarget(session)
    case session.DATA: // local data
      session := ev.Session.Obj.(*Session)
      if session.targetConn == nil {
        session.sendQueue.In <- ev.Data
      } else {
        session.targetConn.Write(ev.Data)
      }
    case session.CLOSE: // local session closed
      ev.Session.Close()
      defer func() {
        targetConn := ev.Session.Obj.(*Session).targetConn
        if targetConn != nil {
          <-time.After(time.Second * 5)
          targetConn.Close()
        }
      }()
    case session.ERROR: // error
      break loop
    }
  // target connection events
  case sessionI := <-targetConnEvents.Out:
    session := sessionI.(*Session)
    readQueue: for { select {
    case dataI := <-session.sendQueue.Out:
      data := dataI.([]byte)
      session.targetConn.Write(data)
    default: break readQueue
    }}
  // target events
  case evI := <-targetReader.Events.Out:
    ev := evI.(cr.Event)
    session := ev.Obj.(*Session)
    switch ev.Type {
    case cr.DATA:
      session.session.Send(ev.Data)
    case cr.EOF, cr.ERROR:
      session.session.Close()
      session.sendQueue.Close()
    }
  }}
}
