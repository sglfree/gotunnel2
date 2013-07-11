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
    http.ListenAndServe("0.0.0.0:55555", nil)
  }()
}

func main() {
  go func() { // profile
    profileTicker := time.NewTicker(time.Second * 30)
    for _ = range profileTicker.C {
      outfile, err := os.Create("server_mem_prof")
      if err != nil { log.Fatal(err) }
      pprof.WriteHeapProfile(outfile)
      outfile.Close()
    }
  }()

  addr, err := net.ResolveTCPAddr("tcp", globalConfig["listen"])
  if err != nil { log.Fatal("cannot resolve listen address ", err) }
  ln, err := net.ListenTCP("tcp", addr)
  if err != nil { log.Fatal("cannot listen ", err) }
  fmt.Printf("server listening on %v\n", ln.Addr())
  connChangeChans := make(map[int64]chan *net.TCPConn)
  for {
    conn, err := ln.AcceptTCP()
    if err != nil { continue }
    go func() {
      var commId int64
      err = binary.Read(conn, binary.LittleEndian, &commId)
      if err != nil { return }
      c, ok := connChangeChans[commId]
      if ok { // change conn
        fmt.Printf("resetting conn of %d\n", commId)
        c <- conn
      } else {
        connChangeChans[commId] = make(chan *net.TCPConn, 8)
        startServ(conn, connChangeChans[commId])
      }
    }()
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
  targetReader := cr.New()
  comm := session.NewComm(conn, []byte(globalConfig["key"]), nil)
  targetConnEvents := make(chan *Serv, 512)
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

  heartbeat := time.NewTicker(time.Minute * 5)

  loop: for { select {
  // heartbeat
  case <-heartbeat.C:
    if time.Now().Sub(comm.LastReadTime) > time.Minute * 5 {
      break loop
    }
  // conn change
  case conn := <-connChange:
    comm = session.NewComm(conn, []byte(globalConfig["key"]), comm)
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
  }}

  // clear
  for _, session := range comm.Sessions {
    serv, ok := session.Obj.(*Serv)
    if ok {
      closeServ(serv)
      fmt.Printf("closed serv %d\n", serv.session.Id)
    } else {
      fmt.Printf("closed session %d\n", session.Id)
      session.Close()
    }
  }
  comm.Close()
  fmt.Printf("comm closed\n")
}

func closeServ(serv *Serv) {
  serv.session.Close()
  close(serv.sendQueue)
}
