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
}

func handleClient(conn *net.TCPConn) {
  defer conn.Close()

  targetReader := cr.New()
  comm := session.NewComm(conn)

  loop: for { select {
  // messages from local 
  case msg := <-comm.Messages:
    switch msg.Type {
    case session.SESSION: // new local session
      session := &Session{
        sendQueue: ic.New(),
      }
      session.session = comm.NewSession(msg.Session.Id, nil, session)
      go func() {
        addr, err := net.ResolveTCPAddr("tcp", string(msg.Data))
        hostError := false
        if err != nil { hostError = true }
        targetConn, err := net.DialTCP("tcp", nil, addr)
        if err != nil { hostError = true }
        if !hostError {
          targetReader.Add(targetConn, session)
          session.targetConn = targetConn
        }
        for {
          data := (<-session.sendQueue.Out).([]byte)
          if !hostError { targetConn.Write(data) }
        }
      }()
    case session.DATA: // local data
      msg.Session.Obj.(*Session).sendQueue.In <- msg.Data
    case session.CLOSE: // local session closed
      msg.Session.Close()
      defer func() {
        targetConn := msg.Session.Obj.(*Session).targetConn
        if targetConn != nil {
          <-time.After(time.Second * 5)
          targetConn.Close()
        }
      }()
    case session.ERROR: // error
      break loop
    }
  // messages from target host
  case msgI := <-targetReader.Messages.Out:
    msg := msgI.(cr.Message)
    session := msg.Obj.(*Session)
    switch msg.Type {
    case cr.DATA:
      session.session.Send(msg.Data)
    case cr.EOF, cr.ERROR:
      session.session.Close()
      session.sendQueue.Close()
    }
  }}
}
