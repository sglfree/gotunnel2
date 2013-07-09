package main

import (
  "log"
  "fmt"
  "./socks"
  cr "./conn_reader"
  "net"
  "./session"
  "time"
  "math/rand"
  "bytes"
)

// configuration
var defaultConfig = map[string]string{
  "local": "localhost:23456",
  "remote": "localhost:34567",
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
  checkConfig("local")
  checkConfig("remote")
  checkConfig("key")

  rand.Seed(time.Now().UnixNano())
}

type Serv struct {
  session *session.Session
  clientConn *net.TCPConn
  hostPort string
  localClosed bool
  remoteClosed bool
}

func main() {
  t1 := time.Now()
  delta := func() string {
    return fmt.Sprintf("%-15v", time.Now().Sub(t1))
  }

  // socks5 server
  socksServer, err := socks.New(globalConfig["local"])
  if err != nil {
    log.Fatal(err)
  }
  fmt.Printf("socks server listening on %s\n", globalConfig["local"])
  clientReader := cr.New()

  // connect to remote server
  addr, err := net.ResolveTCPAddr("tcp", globalConfig["remote"])
  if err != nil { log.Fatal("cannot resolve remote addr ", err) }
  serverConn, err := net.DialTCP("tcp", nil, addr)
  if err != nil { log.Fatal("cannot connect to remote server ", err) }
  defer serverConn.Close()
  fmt.Printf("connected to server %v\n", serverConn.RemoteAddr())
  comm := session.NewComm(serverConn, []byte(globalConfig["key"]))

  // keepalive
  keepaliveSession := comm.NewSession(-1, []byte(keepaliveSessionMagic), nil)
  keepaliveTicker := time.NewTicker(time.Second * 5)

  // obfuscation
  obfusSession := comm.NewSession(-1, []byte(obfusSessionMagic), nil)
  obfusTimer := time.NewTimer(time.Millisecond * time.Duration(rand.Intn(2000) + 500))

  for { select {
  // obfuscation
  case <-obfusTimer.C:
    obfusSession.Send(bytes.Repeat([]byte(fmt.Sprintf("%s", time.Now().UnixNano())), rand.Intn(1024)))
    obfusTimer = time.NewTimer(time.Millisecond * time.Duration(rand.Intn(2000) + 500))
  // keepalive
  case <-keepaliveTicker.C:
    keepaliveSession.Signal(sigPing)
    fmt.Printf("%s ping\n", delta())
  // new socks client
  case socksClient := <-socksServer.Clients:
    serv := &Serv{
      clientConn: socksClient.Conn,
      hostPort: socksClient.HostPort,
    }
    serv.session = comm.NewSession(-1, []byte(socksClient.HostPort), serv)
    clientReader.Add(socksClient.Conn, serv)
  // client events
  case ev := <-clientReader.Events:
    serv := ev.Obj.(*Serv)
    switch ev.Type {
    case cr.DATA: // client data
      serv.session.Send(ev.Data)
    case cr.EOF, cr.ERROR: // client close
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed { serv.session.Close() }
    }
  // server events
  case ev := <-comm.Events:
    switch ev.Type {
    case session.SESSION:
      log.Fatal("local should not have received this type of event")
    case session.DATA:
      serv := ev.Session.Obj.(*Serv)
      serv.clientConn.Write(ev.Data)
    case session.SIGNAL:
      sig := ev.Data[0]
      if sig == sigClose {
        serv := ev.Session.Obj.(*Serv)
        go func() {
          <-time.After(time.Second * 5)
          serv.clientConn.Close()
        }()
        serv.remoteClosed = true
        if serv.localClosed { serv.session.Close() }
      } else if sig == sigPing {
        fmt.Printf("%s pong\n", delta())
      }
    case session.ERROR:
      log.Fatal("error when communicating with server ", string(ev.Data))
    }
  }}
}
