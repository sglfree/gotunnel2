package main

import (
  "log"
  "fmt"
  "./socks"
  cr "./conn_reader"
  "net"
)

var defaultConfig = map[string]string{
  "local": "localhost:23456",
  "remote": "localhost:34567",
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
  checkConfig("local")
  checkConfig("remote")
}

func main() {
  socksServer, err := socks.New(globalConfig["local"])
  if err != nil {
    log.Fatal(err)
  }
  fmt.Printf("socks server listening on %s\n", globalConfig["local"])

  clientReader := cr.New()
  sessions := make(map[int64]*Session)

  addr, err := net.ResolveTCPAddr("tcp", globalConfig["remote"])
  if err != nil { log.Fatal("cannot resolve remote addr ", err) }
  serverConn, err := net.DialTCP("tcp", nil, addr)
  if err != nil { log.Fatal("cannot connect to remote server ", err) }
  defer serverConn.Close()
  fmt.Printf("connected to server %v\n", serverConn.RemoteAddr())
  remoteMessages := startRemoteReader(sessions, serverConn)

  loop: for {
    select {
    case socksClientI := <-socksServer.Clients.Out:
      socksClient := socksClientI.(*socks.Client)
      info := clientReader.Add(socksClient.Conn, -1)
      session := &Session{
        Id: info.Id,
        RemoteConn: serverConn,
        LocalConn: socksClient.Conn,
        HostPort: socksClient.HostPort,
      }
      sessions[session.Id] = session
      err := session.Connect()
      if err != nil {
        fmt.Printf("error when session connect: %s\n", err.Error())
        break loop
      }
      session.log("created", session.HostPort)
    case msgI := <-clientReader.Messages.Out:
      msg := msgI.(cr.Message)
      session, ok := sessions[msg.Info.Id]
      if !ok { continue loop }
      switch msg.Type {
      case cr.DATA:
        session.Write(msg.Data)
        session.log("client >>", fmt.Sprintf("%6d", len(msg.Data)), ">> remote")
      case cr.EOF, cr.ERROR:
        session.log("client >> EOF")
        session.WriteHeader(EOF)
        session.localClosed = true
        if session.remoteClosed {
          delete(sessions, session.Id)
          session.log("cleared")
        }
      }
    case msgI := <-remoteMessages.Out:
      msg := msgI.(Message)
      switch msg.Type {
      case DATA:
        msg.Session.LocalConn.Write(msg.Data)
        msg.Session.log("remote >>", fmt.Sprintf("%6d", len(msg.Data)), ">> client")
      case ERROR:
        break loop
      default:
        log.Fatal("not here")
      }
    }
  }

  fmt.Printf("server disconnected\n")
}
