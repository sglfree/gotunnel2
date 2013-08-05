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

  BUFFERED_CHAN_SIZE = 4096
)

type Event struct {
  Type int
  Session *Session
  Data []byte
}

type Comm struct {
  IsClosed bool
  conn *net.TCPConn // tcp connection to other side
  Sessions map[int64]*Session // map session id to *Session
  readyToSend chan *Session
  ackQueue chan []byte // ack packet queue
  Events chan Event // events channel
  serial uint64 // next packet serial
  maxReceivedSerial uint64
  maxAckSerial uint64
  key []byte // encryption key
  BytesSent uint64
  BytesReceived uint64
  packets *RingQueue // packet buffer
  stopSender chan struct{} // chan to stop sender
  stopAck chan struct{} // chan to stop ack
  stoppedReader chan struct{}
  stoppedSender chan struct{}
  stoppedAck chan struct{}
  LastReadTime time.Time
}

type Packet struct {
  serial uint64
  data []byte
  next *Packet
}

func NewComm(conn *net.TCPConn, key []byte, ref *Comm) (*Comm) {
  if ref != nil && !ref.IsClosed { ref.Close() }
  c := new(Comm)
  c.conn = conn
  if ref != nil && ref.Sessions != nil {
    c.Sessions = ref.Sessions
    for _, session := range c.Sessions {
      session.comm = c
    }
  } else {
    c.Sessions = make(map[int64]*Session)
  }
  c.readyToSend = make(chan *Session, BUFFERED_CHAN_SIZE)
  c.ackQueue = make(chan []byte, BUFFERED_CHAN_SIZE)
  if ref != nil && ref.Events != nil {
    c.Events = ref.Events
  } else {
    c.Events = make(chan Event, BUFFERED_CHAN_SIZE)
  }
  if ref != nil && ref.serial != 0 {
    c.serial = ref.serial
  }
  if ref != nil && ref.maxReceivedSerial != 0 {
    c.maxReceivedSerial = ref.maxReceivedSerial
  }
  if ref != nil && ref.maxAckSerial != 0 {
    c.maxAckSerial = ref.maxAckSerial
  }
  c.key = key
  _, err := aes.NewCipher(c.key)
  if err != nil { log.Fatal(err) }
  if ref != nil && ref.BytesSent != 0 {
    c.BytesSent = ref.BytesSent
  }
  if ref != nil && ref.BytesReceived != 0 {
    c.BytesReceived = ref.BytesReceived
  }
  if ref != nil && ref.packets != nil {
    c.packets = ref.packets
  } else {
    c.packets = NewRing()
  }
  c.stopSender = make(chan struct{})
  c.stopAck = make(chan struct{})
  c.stoppedReader = make(chan struct{})
  c.stoppedSender = make(chan struct{})
  c.stoppedAck = make(chan struct{})
  c.LastReadTime = time.Now()

  // resent not acked packet
  if ref != nil && ref.packets != nil {
    for p := ref.packets.tail; p != ref.packets.head; p = p.next {
      c.conn.Write(p.data)
      c.BytesSent += uint64(len(p.data))
    }
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
  case session := <-self.readyToSend:
    packet := <-session.sendQueue
    self.conn.Write(packet.data)
    self.BytesSent += uint64(len(packet.data))
    self.packets.Enqueue(packet.serial, packet.data)
  case data := <-self.ackQueue:
    self.conn.Write(data)
    self.BytesSent += uint64(len(data))
  case <-self.stopSender:
    close(self.stoppedSender)
    return
  }}
}

func (self *Comm) startReader() {
  defer close(self.stoppedReader)
  var id int64
  var t uint8
  var dataLen uint32
  var serial uint64
  var err error
  loop: for {
    // read header
    err = binary.Read(self.conn, binary.LittleEndian, &serial)
    if err != nil { return }
    self.BytesReceived += 8
    err = binary.Read(self.conn, binary.LittleEndian, &id)
    if err != nil { return }
    self.BytesReceived += 8
    err = binary.Read(self.conn, binary.LittleEndian, &t)
    if err != nil { return }
    self.BytesReceived += 1
    self.LastReadTime = time.Now() // update last read time
    // is ack packet
    if t == typeAck {
      self.maxAckSerial = serial
      // clear packet buffer
      for p := self.packets.Peek(); p != nil && p.serial <= serial; {
        self.packets.Dequeue()
        p = self.packets.Peek()
      }
      continue loop
    }
    // read data
    err = binary.Read(self.conn, binary.LittleEndian, &dataLen)
    if err != nil { return }
    self.BytesReceived += 4
    data := make([]byte, dataLen)
    n, err := io.ReadFull(self.conn, data)
    if err != nil || uint32(n) != dataLen {
      return
    }
    self.BytesReceived += uint64(n)
    if serial <= self.maxReceivedSerial { continue loop } // duplicated packet
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
      session, ok := self.Sessions[id]
      if !ok {
        self.emit(Event{Type: ERROR, Session: &Session{Id: id}, Data: []byte("unregistered session id")})
        return
      }
      self.emit(Event{Type: DATA, Session: session, Data: data})
    case typeSignal:
      session, ok := self.Sessions[id]
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
  ticker := time.NewTicker(time.Millisecond * 500)
  buf := new(bytes.Buffer)
  loop: for { select {
  case <-ticker.C:
    ackSerial := self.maxReceivedSerial
    if ackSerial == lastAck { continue loop }
    buf.Reset()
    binary.Write(buf, binary.LittleEndian, ackSerial)
    binary.Write(buf, binary.LittleEndian, rand.Int63())
    binary.Write(buf, binary.LittleEndian, typeAck)
    self.ackQueue <- buf.Bytes()
    lastAck = ackSerial
  case <-self.stopAck:
    close(self.stoppedAck)
    return
  }}
}

func (self *Comm) Close() {
  self.conn.Close()
  close(self.stopSender)
  close(self.stopAck)
  <-self.stoppedReader
  <-self.stoppedSender
  <-self.stoppedAck
  close(self.readyToSend)
  close(self.ackQueue)
  self.IsClosed = true
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
    sendQueue: make(chan Packet, 512),
  }
  if isNew {
    session.sendPacket(typeConnect, data)
    self.readyToSend <- session
  }
  self.Sessions[id] = session
  return session
}
