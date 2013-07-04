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

  var serverConn *net.TCPConn
  go func() {
    addr, err := net.ResolveTCPAddr("tcp", globalConfig["remote"])
    if err != nil { log.Fatal("cannot resolve remote addr ", err) }
    serverConn, err = net.DialTCP("tcp", nil, addr)
    if err != nil { log.Fatal("cannot connect to remote server ", err) }
    fmt.Printf("connected to server %v\n", serverConn.RemoteAddr())
  }()

  sessions := make(map[int64]*Session)
  for {
    select {
    case socksClientI := <-socksServer.Clients.Out:
      socksClient := socksClientI.(*socks.Client)
      info := clientReader.Add(socksClient.Conn, -1)
      session := &Session{
        Id: info.Id,
        Conn: serverConn,
        HostPort: socksClient.HostPort,
      }
      sessions[session.Id] = session
      err := session.Connect()
      if err != nil { log.Fatal(err) }
    case msgI := <-clientReader.Messages.Out:
      handleClientMsg(msgI.(cr.Message))
    }
  }

}

func handleClientMsg(msg cr.Message) {
  switch msg.Type {
  case cr.DATA:
    //fmt.Printf("=> %s\n", msg.Data[:128])
  case cr.EOF:
    msg.Info.TCPConn.Close()
  case cr.ERROR:
    log.Fatal(msg.Data)
  }
}
