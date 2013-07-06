package session

import (
  "net"
  "math/rand"
  "time"
  "encoding/binary"
  "log"
  "io"
  "fmt"
)

func init() {
  rand.Seed(time.Now().UnixNano())
}

const (
  SESSION = iota
  DATA
  CLOSE
  ERROR
)

type Message struct {
  Type int
  Session *Session
  Data []byte
}

type Comm struct {
  conn *net.TCPConn
  sessions map[int64]*Session
  sendQueue chan []byte
  Messages chan Message
}

func NewComm(conn *net.TCPConn) (*Comm) {
  c := &Comm{
    conn: conn,
    sessions: make(map[int64]*Session),
    sendQueue: make(chan []byte),
    Messages: make(chan Message),
  }
  go c.startSender()
  go c.startReader()
  return c
}

func (self *Comm) startSender() {
  for {
    data := <-self.sendQueue
    self.conn.Write(data)
  }
}

func (self *Comm) startReader() {
  var id int64
  var t uint8
  var dataLen uint32
  for {
    binary.Read(self.conn, binary.LittleEndian, &id)
    binary.Read(self.conn, binary.LittleEndian, &t)
    binary.Read(self.conn, binary.LittleEndian, &dataLen)
    data := make([]byte, dataLen)
    n, err := io.ReadFull(self.conn, data)
    if err != nil || uint32(n) != dataLen {
      self.provideMessage(Message{Type: ERROR, Data: []byte("error occurred when reading data")})
      return
    }
    switch t {
    case typeConnect:
      session := self.NewSession(id, nil, nil)
      self.provideMessage(Message{Type: SESSION, Session: session, Data: data})
    case typeData:
      session, ok := self.sessions[id]
      if !ok {
        self.provideMessage(Message{Type: ERROR, Session: &Session{Id: id}, Data: []byte("unregistered session id")})
        return
      }
      self.provideMessage(Message{Type: DATA, Session: session, Data: data})
    case typeClose:
      session, ok := self.sessions[id]
      if !ok {
        self.provideMessage(Message{Type: ERROR, Session: &Session{Id: id}, Data: []byte("unregistered session id")})
        return
      }
      self.provideMessage(Message{Type: CLOSE, Session: session})
    default:
      self.provideMessage(Message{Type: ERROR, Data: []byte(fmt.Sprintf("unrecognized packet type %s", t))})
      return
    }
  }
}

func (self *Comm) provideMessage(msg Message) {
  select {
  case self.Messages <- msg:
  case <-time.After(time.Second * 5):
    log.Fatal("no one is reading the message, check your code!")
  }
}

func (self *Comm) NewSession(id int64, data []byte, obj interface{}) (*Session) {
  isNew := false
  if id <= int64(0) {
    isNew = true
    id = rand.Int63()
  }
  session := &Session{
    Id: id,
    comm: self,
    Obj: obj,
  }
  if isNew {
    self.sendQueue <- session.constructPacket(typeConnect, data)
  }
  self.sessions[id] = session
  return session
}
