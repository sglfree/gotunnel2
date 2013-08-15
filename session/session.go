package session

import (
  "encoding/binary"
  "bytes"
  "crypto/aes"
  "sync/atomic"
  "time"
)

const OLD_SESSION_DATA_SENT = 1024 * 1024 * 4

type Session struct {
  Id int64
  comm *Comm
  Obj interface{}
  sendQueue chan *Packet
  dataSent uint64
  serial uint32 // next packet serial
  maxReceivedSerial uint32
  maxAckSerial uint32
  packets *Queue // packet buffer
  StartTime time.Time
}

func (self *Session) nextSerial() uint32 {
  return atomic.AddUint32(&(self.serial), uint32(1))
}

func (self *Session) sendPacket(t uint8, data []byte) {
  buf := new(bytes.Buffer)
  buf.Grow(len(data) + 4 + 8 + 1)
  serial := self.nextSerial()
  binary.Write(buf, binary.LittleEndian, serial)
  binary.Write(buf, binary.LittleEndian, self.Id)
  binary.Write(buf, binary.LittleEndian, t)
  block, _ := aes.NewCipher(self.comm.key)
  v := make([]byte, aes.BlockSize)
  var i, size int
  for i, size = aes.BlockSize, len(data); i < size; i += aes.BlockSize {
    block.Encrypt(v, data[i - aes.BlockSize : i])
    buf.Write(v)
  }
  buf.Write(data[i - aes.BlockSize :])
  packet := &Packet{serial: serial, data: buf.Bytes(), createTime: time.Now()}
  self.sendQueue <- packet
  self.dataSent += uint64(len(data))
}

func (self *Session) Send(data []byte) {
  self.sendPacket(typeData, data)
  if self.dataSent > OLD_SESSION_DATA_SENT {
    self.comm.readyToSend2 <- self
  } else if self.dataSent > 1024 * 16 {
    self.comm.readyToSend1 <- self
  } else {
    self.comm.readyToSend0 <- self
  }
  self.comm.readySigIn <- struct{}{}
}

func (self *Session) Signal(sig uint8) {
  self.sendPacket(typeSignal, []byte{sig})
  self.comm.readyToSend0 <- self
  self.comm.readySigIn <- struct{}{}
}

func (self *Session) Close() {
  close(self.sendQueue)
  delete(self.comm.Sessions, self.Id)
}
