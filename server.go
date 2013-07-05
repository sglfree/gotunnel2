package main

import (
  "net"
  "log"
  "fmt"
  cr "./conn_reader"
  ic "./infinite_chan"
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

  targetReader := cr.New()
  sessions := make(map[int64]*Session)
  remoteMessages := startRemoteReader(sessions, conn)

  loop: for {
    select {
    case msgI := <-targetReader.Messages.Out:
      msg := msgI.(cr.Message)
      session, ok := sessions[msg.Info.Id]
      if !ok { continue loop }
      switch msg.Type {
      case cr.DATA:
        session.Write(msg.Data)
        session.log("target >>", fmt.Sprintf("%6d", len(msg.Data)), ">> remote")
      case cr.EOF, cr.ERROR:
        session.log("target >> EOF")
        session.WriteHeader(EOF)
        session.localClosed = true
        if session.remoteClosed {
          if session.SendQueue != nil {
            session.SendQueue.Close()
          }
          delete(sessions, session.Id)
          session.log("cleared")
        }
      }
    case msgI := <-remoteMessages.Out:
      msg := msgI.(Message)
      switch msg.Type {
      case CONNECT:
        session := msg.Session
        addr, err := net.ResolveTCPAddr("tcp", session.HostPort)
        if err != nil {
          session.log("cannot resolve addr", session.HostPort, err)
          continue loop
        }
        session.SendQueue = ic.New()
        go func() {
          targetConn, err := net.DialTCP("tcp", nil, addr)
          if err != nil {
            session.log("cannot connect to host", session.HostPort, err)
            return
          }
          targetReader.Add(targetConn, session.Id)
          session.LocalConn = targetConn
          session.log("connected to target host", session.HostPort)
          for {
            data := (<-session.SendQueue.Out).([]byte)
            session.LocalConn.Write(data)
          }
        }()
      case DATA:
        msg.Session.SendQueue.In <- msg.Data
        msg.Session.log("remote >>", fmt.Sprintf("%6d", len(msg.Data)), ">> target")
      case ERROR:
        break loop
      default:
        log.Fatal("not here")
      }
    }
  }

  fmt.Printf("client disconnected\n")
}
