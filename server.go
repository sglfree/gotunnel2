package main

import (
  "net"
  "log"
  "fmt"
  cr "./conn_reader"
  "./session"
  "time"
  //"io/ioutil"
  "io"
  "os"
  "runtime/pprof"
  _ "net/http/pprof"
  "net/http"
  "encoding/binary"
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
  go func() {
    http.ListenAndServe("localhost:55555", nil)
  }()
}

func main() {
  addr, err := net.ResolveTCPAddr("tcp", globalConfig["listen"])
  if err != nil { log.Fatal("cannot resolve listen address ", err) }
  ln, err := net.ListenTCP("tcp", addr)
  if err != nil { log.Fatal("cannot listen ", err) }
  fmt.Printf("server listening on %v\n", ln.Addr())
  var commId int64
  connChangeChans := make(map[int64]chan *net.TCPConn)
  for {
    conn, err := ln.AcceptTCP()
    if err != nil { continue }
    err = binary.Read(conn, binary.LittleEndian, &commId)
    if err != nil { continue }
    c, ok := connChangeChans[commId]
    if ok { // change conn
      fmt.Printf("resetting conn of %d\n", commId)
      c <- conn
    } else {
      connChangeChans[commId] = make(chan *net.TCPConn)
      go startServ(conn, connChangeChans[commId])
    }
  }
}

type Serv struct {
  session *session.Session
  sendQueue chan []byte
  targetConn io.Writer
  localClosed bool
  remoteClosed bool
}

func startServ(conn *net.TCPConn, connChange chan *net.TCPConn) {
  t1 := time.Now()
  delta := func() string {
    return fmt.Sprintf("%-10.3f", time.Now().Sub(t1).Seconds())
  }

  targetReader := cr.New()
  comm := session.NewComm(conn, []byte(globalConfig["key"]), nil)
  targetConnEvents := make(chan *Serv, 65536)
  connectTarget := func(serv *Serv, hostPort string) {
    defer func() {
      targetConnEvents <- serv
    }()
    addr, err := net.ResolveTCPAddr("tcp", hostPort)
    if err != nil { return }
    targetConn, err := net.DialTCP("tcp", nil, addr)
    if err != nil { return }
    targetReader.Add(targetConn, serv)
    serv.targetConn = targetConn
    return
  }

  profileTicker := time.NewTicker(time.Second * 30)

  loop: for { select {
  // conn change
  case conn := <-connChange:
    comm = session.NewComm(conn, []byte(globalConfig["key"]), comm)
  // write heap profile
  case <-profileTicker.C:
    outfile, err := os.Create("server_mem_prof")
    if err != nil { log.Fatal(err) }
    pprof.WriteHeapProfile(outfile)
    outfile.Close()
  // local-side events
  case ev := <-comm.Events:
    switch ev.Type {
    case session.SESSION: // new local session
      hostPort := string(ev.Data)
      if hostPort == keepaliveSessionMagic {
        continue loop
      }
      serv := &Serv{
        sendQueue: make(chan []byte, 8),
      }
      serv.session = ev.Session
      ev.Session.Obj = serv
      go connectTarget(serv, hostPort)
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
          time.AfterFunc(time.Second * 5, func() {
            serv.targetConn.(*net.TCPConn).Close()
          })
        }
        serv.remoteClosed = true
        if serv.localClosed { closeServ(serv) }
      } else if sig == sigPing { // from keepaliveSession
        fmt.Printf("%s pong %10d >< %-10d\n", delta(), comm.BytesSent, comm.BytesReceived)
        ev.Session.Signal(sigPing)
      }
    case session.ERROR: // error
      break loop
    }
  // target connection events
  case serv := <-targetConnEvents:
    if serv.targetConn == nil { // fail to connect to target
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed { closeServ(serv) }
      continue loop
    }
    go func() {
      for data := range serv.sendQueue {
        serv.targetConn.Write(data)
      }
    }()
  // target events
  case ev := <-targetReader.Events:
    serv := ev.Obj.(*Serv)
    switch ev.Type {
    case cr.DATA:
      serv.session.Send(ev.Data)
    case cr.EOF, cr.ERROR:
      serv.session.Signal(sigClose)
      serv.localClosed = true
      if serv.remoteClosed { closeServ(serv) }
    }
  goto loop
  }}
}

func closeServ(serv *Serv) {
  serv.session.Close()
  close(serv.sendQueue)
}
