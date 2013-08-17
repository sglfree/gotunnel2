package main

import (
  box "github.com/nsf/termbox-go"
  "log"
  "fmt"
  "./socks"
  cr "./conn_reader"
  "net"
  "./session"
  "time"
  "math/rand"
  "encoding/binary"
  _ "net/http/pprof"
  "net/http"
  "os"
  "reflect"
  "runtime"
  "sync"
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
  go func() {
    http.ListenAndServe("0.0.0.0:55556", nil)
  }()
}

type Serv struct {
  session *session.Session
  clientConn *net.TCPConn
  hostPort string
  localClosed bool
  remoteClosed bool
  closeOnce sync.Once
}

func main() {
  // log stack
  defer func() {
    if r := recover(); r != nil {
      buf := make([]byte, 1024 * 1024 * 8)
      n := runtime.Stack(buf, true)
      errFile, err := os.Create("err.local")
      if err != nil { return }
      errFile.WriteString(fmt.Sprintf("error: %v\n\n", r))
      errFile.Write(buf[:n])
      errFile.Close()
    }
  }()
  // termbox
  err := box.Init()
  if err != nil { log.Fatal(err) }
  defer box.Close()
  go func() { for {
    ev := box.PollEvent()
    if ev.Type == box.EventKey {
      if ev.Key == box.KeyEsc {
        os.Exit(0)
      }
    }
  }}()
  // socks5 server
  socksServer, err := socks.New(globalConfig["local"])
  if err != nil {
    log.Fatal(err)
  }
  clientReader := cr.New()

  // connect to remote server
  addr, err := net.ResolveTCPAddr("tcp", globalConfig["remote"])
  if err != nil { log.Fatal("cannot resolve remote addr ", err) }
  commId := rand.Int63()
  cipherKey := []byte(globalConfig["key"])
  getServerConn := func() *net.TCPConn {
    serverConn, err := net.DialTCP("tcp", nil, addr)
    if err != nil { log.Fatal("cannot connect to remote server ", err) }
    // auth
    origin := genRandBytes(64)
    encrypted, err := encrypt(cipherKey, origin)
    if err != nil { log.Fatal(err) }
    serverConn.Write(origin)
    serverConn.Write(encrypted)
    var response byte
    err = binary.Read(serverConn, binary.LittleEndian, &response)
    if err != nil || response != byte(1) { log.Fatal("auth fail ", err) }
    // sent comm id
    binary.Write(serverConn, binary.LittleEndian, commId)
    return serverConn
  }
  comm := session.NewComm(getServerConn(), cipherKey)

  // keepalive
  keepaliveSession := comm.NewSession(-1, []byte(keepaliveSessionMagic), nil)
  keepaliveTicker := time.NewTicker(PING_INTERVAL)

  // heartbeat
  heartbeat := time.NewTicker(time.Second * 1)
  t1 := time.Now()
  delta := func() string {
    return fmt.Sprintf("%-.0fs", time.Now().Sub(t1).Seconds())
  }

  var memStats runtime.MemStats
  printer := NewPrinter(40)
  reconnectTimes := 0

  loop: for { select {
  // ping
  case <-keepaliveTicker.C:
    keepaliveSession.Signal(sigPing)
  // heartbeat
  case <-heartbeat.C:
    if time.Now().Sub(comm.LastReadTime) > BAD_CONN_THRESHOLD {
      comm.UseConn(getServerConn())
      reconnectTimes += 1
    }

    box.Clear(box.ColorDefault, box.ColorDefault)
    printer.Reset()
    printer.Print("conf %s", CONFIG_FILEPATH)
    printer.Print("listening %v", globalConfig["local"])
    printer.Print("connected %v", addr)
    printer.Print("reconnected %d times", reconnectTimes)
    printer.Print("%s %s >-< %s", delta(), formatFlow(comm.BytesSent), formatFlow(comm.BytesReceived))
    runtime.ReadMemStats(&memStats)
    printer.Print("%s memory in use", formatFlow(memStats.Alloc))
    printer.Print("%d connections", clientReader.Count)
    printer.Print("--- %d sessions ---", len(comm.Sessions))
    for _, sessionId := range ByValue(comm.Sessions, func(a, b reflect.Value) bool {
      return a.Interface().(*session.Session).StartTime.After(b.Interface().(*session.Session).StartTime)
    }).Interface().([]int64) {
      session := comm.Sessions[sessionId]
      serv, ok := session.Obj.(*Serv)
      if !ok { continue }
      if serv.localClosed {
        printer.Print("Lx %s", serv.hostPort)
      } else if serv.remoteClosed {
        printer.Print("Rx %s", serv.hostPort)
      } else {
        printer.Print(serv.hostPort)
      }
    }
    box.Flush()

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
      if serv.session == nil { // serv already closed
        continue loop
      }
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed {
        serv.Close()
      } else {
        time.AfterFunc(time.Minute * 3, func() { serv.Close() })
      }
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
        time.AfterFunc(time.Second * 3, func() {
          serv.clientConn.Close()
        })
        serv.remoteClosed = true
        if serv.localClosed {
          serv.Close()
        } else {
          time.AfterFunc(time.Minute * 3, func() { serv.Close() })
        }
      } else if sig == sigPing {
      }
    case session.ERROR:
      log.Fatal("error when communicating with server ", string(ev.Data))
    }
  }}
}

func (self *Serv) Close() {
  self.closeOnce.Do(func() {
    self.session.Close()
    self.session = nil
  })
}
