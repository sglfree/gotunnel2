package session

import (
  "net"
  "math/rand"
  "time"
  "encoding/binary"
  "io"
  "fmt"
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
  readyToSend0 chan *Session
  readyToSend1 chan *Session
  readyToSend2 chan *Session
  readySig chan struct{}
  ackQueue chan []byte // ack packet queue
  Events chan Event // events channel
  key []byte // encryption key
  BytesSent uint64
  BytesReceived uint64
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
  createTime time.Time
  sentTime time.Time
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
  c.readyToSend0 = make(chan *Session, BUFFERED_CHAN_SIZE)
  c.readyToSend1 = make(chan *Session, BUFFERED_CHAN_SIZE)
  c.readyToSend2 = make(chan *Session, BUFFERED_CHAN_SIZE)
  c.readySig = make(chan struct{}, BUFFERED_CHAN_SIZE)
  c.ackQueue = make(chan []byte, BUFFERED_CHAN_SIZE)
  if ref != nil && ref.Events != nil {
    c.Events = ref.Events
  } else {
    c.Events = make(chan Event, BUFFERED_CHAN_SIZE)
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
  c.stopSender = make(chan struct{})
  c.stopAck = make(chan struct{})
  c.stoppedReader = make(chan struct{})
  c.stoppedSender = make(chan struct{})
  c.stoppedAck = make(chan struct{})
  c.LastReadTime = time.Now()

  // resent not acked packet
  for _, session := range c.Sessions {
    for t, h := session.packets.tail, session.packets.head; t != h; t = t.next {
      c.conn.Write(t.data)
      c.BytesSent += uint64(len(t.data))
    }
  }

  go c.startSender()
  go c.startReader()
  go c.startAck()

  return c
}

func (self *Comm) sentSessionPacket(session *Session) {
  packet := <-session.sendQueue
  packet.sentTime = time.Now()
  self.conn.Write(packet.data)
  self.BytesSent += uint64(len(packet.data))
  session.packets.En(packet)
}

func (self *Comm) startSender() {
  next: for {
    <-self.readySig
    select {
    case data := <-self.ackQueue:
      self.conn.Write(data)
      self.BytesSent += uint64(len(data))
      continue next
    default:
      select {
      case session := <-self.readyToSend0:
        self.sentSessionPacket(session)
        continue next
      default:
        select {
        case session := <-self.readyToSend1:
          self.sentSessionPacket(session)
          continue next
        default:
          select {
          case session := <-self.readyToSend2:
            self.sentSessionPacket(session)
            continue next
          default:
            select {
            case <-self.stopSender:
              close(self.stoppedSender)
              return
            default:
            }
          }
        }
      }
    }
  }
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
    // get session
    session, ok := self.Sessions[id]
    if !ok && t == typeConnect { // new session
      session = self.NewSession(id, nil, nil)
    } else if !ok {
      if t == typeAck {
        continue loop
      }
      self.emit(Event{Type: ERROR, Session: &Session{Id: id}, Data: []byte(fmt.Sprintf("%d unregistered session id %d %d", id, t, serial))})
      return
    }
    // is ack packet
    if t == typeAck {
      session.maxAckSerial = serial
      // clear packet buffer
      for p, h := session.packets.tail, session.packets.head; p != h && p.serial <= serial; {
        info("%v %v\n", p.sentTime.Sub(p.createTime), time.Now().Sub(p.sentTime))
        session.packets.De()
        p = session.packets.tail
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
    if serial <= session.maxReceivedSerial { // duplicated packet
      continue loop
    }
    session.maxReceivedSerial = serial
    // decrypt
    block, _ := aes.NewCipher(self.key)
    for i, size := aes.BlockSize, len(data); i < size; i += aes.BlockSize {
      block.Decrypt(data[i - aes.BlockSize : i], data[i - aes.BlockSize : i])
    }

    switch t {
    case typeConnect:
      self.emit(Event{Type: SESSION, Session: session, Data: data})
    case typeData:
      self.emit(Event{Type: DATA, Session: session, Data: data})
    case typeSignal:
      self.emit(Event{Type: SIGNAL, Session: session, Data: data})
    default:
      self.emit(Event{Type: ERROR, Data: []byte(fmt.Sprintf("unrecognized packet type %s", t))})
      return
    }
  }
}

func (self *Comm) startAck() {
  lastAck := make(map[int64]uint64)
  ticker := time.NewTicker(time.Millisecond * 500)
  for { select {
  case <-ticker.C:
    for sessionId, session := range self.Sessions {
      ackSerial := session.maxReceivedSerial
      if ackSerial == lastAck[sessionId] { continue }
      buf := new(bytes.Buffer)
      binary.Write(buf, binary.LittleEndian, ackSerial)
      binary.Write(buf, binary.LittleEndian, sessionId)
      binary.Write(buf, binary.LittleEndian, typeAck)
      self.ackQueue <- buf.Bytes()
      self.readySig <- struct{}{}
      lastAck[sessionId] = ackSerial
    }
  case <-self.stopAck:
    close(self.stoppedAck)
    return
  }}
}

func (self *Comm) Close() {
  self.conn.Close()
  close(self.stopSender)
  self.readySig <- struct{}{}
  close(self.stopAck)
  <-self.stoppedReader
  <-self.stoppedSender
  <-self.stoppedAck
  close(self.readyToSend0)
  close(self.readyToSend1)
  close(self.readyToSend2)
  close(self.readySig)
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
    sendQueue: make(chan *Packet, 512),
    packets: NewQueue(),
  }
  if isNew {
    session.sendPacket(typeConnect, data)
    self.readyToSend0 <- session
    self.readySig <- struct{}{}
  }
  self.Sessions[id] = session
  info("%d new session\n", session.Id)
  return session
}
