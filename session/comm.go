package session

import (
	"../utils"
	"bufio"
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	SESSION = iota
	DATA
	SIGNAL
	ERROR

	MAX_DATA_LENGTH = 1<<16 - 1
)

type Event struct {
	Type    int
	Session *Session
	Data    []byte
}

type Packet struct {
	serial uint32
	data   []byte
	next   *Packet
}

type Comm struct {
	IsClosed      bool
	conn          *net.TCPConn       // tcp connection to other side
	Sessions      map[int64]*Session // map session id to *Session
	ackQueue      <-chan []byte      // ack packet queue
	ackQueueIn    chan []byte        // ack packet queue
	eventsIn      chan Event
	Events        <-chan Event // events channel
	key           []byte       // encryption key
	BytesSent     uint64
	BytesReceived uint64
	stopSender    chan struct{} // chan to stop sender
	stopAck       chan struct{} // chan to stop ack
	stoppedReader chan struct{}
	stoppedSender chan struct{}
	stoppedAck    chan struct{}
	LastReadTime  time.Time
	sendQueue     <-chan *Packet
	sendQueueIn   chan *Packet
}

func NewComm(conn *net.TCPConn, key []byte) *Comm {
	_, err := aes.NewCipher(key)
	if err != nil {
		log.Fatal(err)
	}
	c := &Comm{
		conn:          conn,
		Sessions:      make(map[int64]*Session),
		ackQueueIn:    make(chan []byte),
		eventsIn:      make(chan Event),
		key:           key,
		stopSender:    make(chan struct{}),
		stopAck:       make(chan struct{}),
		stoppedReader: make(chan struct{}),
		stoppedSender: make(chan struct{}),
		stoppedAck:    make(chan struct{}),
		LastReadTime:  time.Now(),
		sendQueueIn:   make(chan *Packet),
	}
	c.Events = utils.MakeChan(c.eventsIn).(<-chan Event)
	c.ackQueue = utils.MakeChan(c.ackQueueIn).(<-chan []byte)
	c.sendQueue = utils.MakeChan(c.sendQueueIn).(<-chan *Packet)

	go c.startReader()
	go c.startSender()
	go c.startAck()

	return c
}

func (self *Comm) UseConn(conn *net.TCPConn) {
	// stop
	self.conn.Close()
	<-self.stoppedReader
	close(self.stopSender)
	<-self.stoppedSender
	close(self.stopAck)
	<-self.stoppedAck
	// resent
	self.conn = conn
	for _, session := range self.Sessions {
		for t, h := session.packets.tail, session.packets.head; t != h; t = t.next {
			self.write(t.data)
			self.BytesSent += uint64(len(t.data))
		}
	}
	// restart
	self.LastReadTime = time.Now()
	self.stopSender = make(chan struct{})
	self.stopAck = make(chan struct{})
	self.stoppedReader = make(chan struct{})
	self.stoppedSender = make(chan struct{})
	self.stoppedAck = make(chan struct{})
	go self.startReader()
	go self.startSender()
	go self.startAck()
}

func (self *Comm) write(data []byte) {
	l := len(data)
	if l > MAX_DATA_LENGTH {
		log.Fatal("data too long")
	}
	binary.Write(self.conn, binary.LittleEndian, uint16(l))
	self.conn.Write(data)
}

func (self *Comm) startSender() {
	for {
		select {
		case data := <-self.ackQueue:
			self.write(data)
			self.BytesSent += uint64(len(data))
		case packet := <-self.sendQueue:
			self.write(packet.data)
			self.BytesSent += uint64(len(packet.data))
		case <-self.stopSender:
			close(self.stoppedSender)
			return
		}
	}
}

func (self *Comm) startReader() {
	defer close(self.stoppedReader)
	var id int64
	var t uint8
	var serial uint32
	var err error
	var packetLen uint16
	discardBuffer := make([]byte, MAX_DATA_LENGTH+1)
	var n int
	connReader := bufio.NewReaderSize(self.conn, 65536)
loop:
	for {
		// read packet
		if packetLen > 0 {
			n, _ = io.ReadFull(connReader, discardBuffer[:packetLen])
			if uint16(n) != packetLen {
				return
			}
		}
		err = binary.Read(connReader, binary.LittleEndian, &packetLen)
		if err != nil {
			return
		}
		// read header
		err = binary.Read(connReader, binary.LittleEndian, &serial)
		if err != nil {
			return
		}
		self.BytesReceived += 4
		packetLen -= 4
		err = binary.Read(connReader, binary.LittleEndian, &id)
		if err != nil {
			return
		}
		self.BytesReceived += 8
		packetLen -= 8
		err = binary.Read(connReader, binary.LittleEndian, &t)
		if err != nil {
			return
		}
		self.BytesReceived += 1
		packetLen -= 1
		self.LastReadTime = time.Now() // update last read time
		// get session
		session, ok := self.Sessions[id]
		if !ok && t == typeConnect { // new session
			session = self.NewSession(id, nil, nil)
		} else if !ok {
			continue loop
		}
		// is ack packet
		if t == typeAck {
			session.maxAckSerial = serial
			// clear packet buffer
			for p, h := session.packets.tail, session.packets.head; p != h && p.serial <= serial; {
				session.packets.De()
				p = session.packets.tail
			}
			continue loop
		}
		// check serial
		if serial <= session.maxReceivedSerial { // duplicated packet
			continue loop
		}
		session.maxReceivedSerial = serial
		// read data
		data := make([]byte, packetLen)
		n, _ = io.ReadFull(connReader, data)
		if uint16(n) != packetLen {
			return
		}
		self.BytesReceived += uint64(packetLen)
		packetLen = 0
		// decrypt
		block, _ := aes.NewCipher(self.key)
		for i, size := aes.BlockSize, len(data); i < size; i += aes.BlockSize {
			block.Decrypt(data[i-aes.BlockSize:i], data[i-aes.BlockSize:i])
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

func (self *Comm) emit(ev Event) {
	self.eventsIn <- ev
}

func (self *Comm) startAck() {
	lastAck := make(map[int64]uint32)
	ticker := time.NewTicker(time.Millisecond * 500)
	for {
		select {
		case <-ticker.C:
			for sessionId, session := range self.Sessions {
				ackSerial := session.maxReceivedSerial
				if ackSerial == lastAck[sessionId] {
					continue
				}
				buf := new(bytes.Buffer)
				binary.Write(buf, binary.LittleEndian, ackSerial)
				binary.Write(buf, binary.LittleEndian, sessionId)
				binary.Write(buf, binary.LittleEndian, typeAck)
				self.ackQueueIn <- buf.Bytes()
				lastAck[sessionId] = ackSerial
			}
		case <-self.stopAck:
			close(self.stoppedAck)
			return
		}
	}
}

func (self *Comm) Close() {
	self.conn.Close()
	close(self.stopSender)
	close(self.stopAck)
	<-self.stoppedReader
	<-self.stoppedSender
	<-self.stoppedAck
	close(self.eventsIn)
	close(self.ackQueueIn)
	close(self.sendQueueIn)
	self.IsClosed = true
}

func (self *Comm) NewSession(id int64, data []byte, obj interface{}) *Session {
	isNew := false
	if id <= int64(0) {
		isNew = true
		id = rand.Int63()
	}
	session := &Session{
		Id:        id,
		comm:      self,
		Obj:       obj,
		packets:   NewQueue(),
		StartTime: time.Now(),
	}
	if isNew {
		session.sendPacket(typeConnect, data)
	}
	self.Sessions[id] = session
	return session
}
