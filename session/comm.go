package session

import (
  "net"
  "math/rand"
  "time"
  "encoding/binary"
  "io"
  "fmt"
  "sync/atomic"
  "bytes"
  "log"
  "crypto/aes"
)

func init() {
  rand.Seed(time.Now().UnixNano())
}

const (
  SESSION = iota
  DATA
  SIGNAL
  ERROR
)

type Event struct {
  Type int
  Session *Session
  Data []byte
}

type Comm struct {
  conn *net.TCPConn
  sessions map[int64]*Session
  sendQueue chan Packet
  ackQueue chan []byte
  Events chan Event
  serial uint64
  maxReceivedSerial uint64
  maxAckSerial uint64
  key []byte
  BytesSent uint64
  BytesReceived uint64
}

type Packet struct {
  serial uint64
  data []byte
  next *Packet
}

func NewComm(conn *net.TCPConn, key []byte) (*Comm) {
  _, err := aes.NewCipher(key)
  if err != nil { log.Fatal(err) }
  c := &Comm{
    conn: conn,
    sessions: make(map[int64]*Session),
    sendQueue: make(chan Packet, 65536),
    ackQueue: make(chan []byte, 65536),
    Events: make(chan Event, 65536),
    key: key,
  }
  go c.startSender()
  go c.startReader()
  go c.startAck()
  return c
}

func (self *Comm) nextSerial() uint64 {
  return atomic.AddUint64(&(self.serial), uint64(1))
}

func (self *Comm) startSender() {
  for { select {
  case packet := <-self.sendQueue:
    self.conn.Write(packet.data)
    self.BytesSent += uint64(len(packet.data))
  case data := <-self.ackQueue:
    self.conn.Write(data)
    self.BytesSent += uint64(len(data))
  }}
}

func (self *Comm) startReader() {
  var id int64
  var t uint8
  var dataLen uint32
  var serial uint64
  loop: for {
    // read header
    binary.Read(self.conn, binary.LittleEndian, &serial)
    self.BytesReceived += 8
    binary.Read(self.conn, binary.LittleEndian, &id)
    self.BytesReceived += 8
    binary.Read(self.conn, binary.LittleEndian, &t)
    self.BytesReceived += 1
    // is ack packet
    if t == typeAck {
      self.maxAckSerial = serial
      continue loop
    }
    // read data
    binary.Read(self.conn, binary.LittleEndian, &dataLen)
    self.BytesReceived += 4
    data := make([]byte, dataLen)
    n, err := io.ReadFull(self.conn, data)
    if err != nil || uint32(n) != dataLen {
      self.emit(Event{Type: ERROR, Data: []byte("error occurred when reading data")})
      return
    }
    self.BytesReceived += uint64(n)
    self.maxReceivedSerial = serial
    // decrypt
    block, _ := aes.NewCipher(self.key)
    for i, size := aes.BlockSize, len(data); i < size; i += aes.BlockSize {
      block.Decrypt(data[i - aes.BlockSize : i], data[i - aes.BlockSize : i])
    }

    switch t {
    case typeConnect:
      session := self.NewSession(id, nil, nil)
      self.emit(Event{Type: SESSION, Session: session, Data: data})
    case typeData:
      session, ok := self.sessions[id]
      if !ok {
        self.emit(Event{Type: ERROR, Session: &Session{Id: id}, Data: []byte("unregistered session id")})
        return
      }
      self.emit(Event{Type: DATA, Session: session, Data: data})
    case typeSignal:
      session, ok := self.sessions[id]
      if !ok {
        self.emit(Event{Type: ERROR, Session: &Session{Id: id}, Data: []byte("unregistered session id")})
        return
      }
      self.emit(Event{Type: SIGNAL, Session: session, Data: data})
    default:
      self.emit(Event{Type: ERROR, Data: []byte(fmt.Sprintf("unrecognized packet type %s", t))})
      return
    }
  }
}

func (self *Comm) startAck() {
  var lastAck uint64
  for {
    <-time.After(time.Millisecond * 500)
    ackSerial := self.maxReceivedSerial
    if ackSerial == lastAck { continue }
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, ackSerial)
    binary.Write(buf, binary.LittleEndian, rand.Int63())
    binary.Write(buf, binary.LittleEndian, typeAck)
    self.ackQueue <- buf.Bytes()
    lastAck = ackSerial
  }
}

func (self *Comm) emit(ev Event) {
  self.Events <- ev
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
