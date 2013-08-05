package session

import (
  "encoding/binary"
  "bytes"
  "crypto/aes"
)

type Session struct {
  Id int64
  comm *Comm
  Obj interface{}
  sendQueue chan Packet
}

func (self *Session) sendPacket(t uint8, data []byte) {
  buf := new(bytes.Buffer)
  buf.Grow(len(data) + 8 + 8 + 1 + 4)
  serial := self.comm.nextSerial()
  binary.Write(buf, binary.LittleEndian, serial)
  binary.Write(buf, binary.LittleEndian, self.Id)
  binary.Write(buf, binary.LittleEndian, t)
  binary.Write(buf, binary.LittleEndian, uint32(len(data)))
  block, _ := aes.NewCipher(self.comm.key)
  v := make([]byte, aes.BlockSize)
  var i, size int
  for i, size = aes.BlockSize, len(data); i < size; i += aes.BlockSize {
    block.Encrypt(v, data[i - aes.BlockSize : i])
    buf.Write(v)
  }
  buf.Write(data[i - aes.BlockSize :])
  packet := Packet{serial: serial, data: buf.Bytes()}
  self.sendQueue <- packet
}

func (self *Session) Send(data []byte) {
  self.sendPacket(typeData, data)
  self.comm.readyToSend <- self
}

func (self *Session) Signal(sig uint8) {
  self.sendPacket(typeSignal, []byte{sig})
  self.comm.readyToSend <- self
}

func (self *Session) Close() {
  close(self.sendQueue)
  delete(self.comm.Sessions, self.Id)
}
