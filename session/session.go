package session

import (
  "encoding/binary"
  "bytes"
  "crypto/aes"
  "time"
)

const OLD_SESSION_SENT_BYTES = 1024 * 1024 * 8

type Session struct {
  Id int64
  comm *Comm
  Obj interface{}
  dataSent uint64
  startTime time.Time
}

func (self *Session) constructPacket(t uint8, data []byte) Packet {
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
  return Packet{serial: serial, data: buf.Bytes()}
}

func (self *Session) Send(data []byte) {
  self.comm.sendSig <- struct{}{}
  if self.dataSent > OLD_SESSION_SENT_BYTES {
    self.comm.sendQueue2 <- self.constructPacket(typeData, data)
  } else if time.Now().Sub(self.startTime) > time.Second * 8 {
    self.comm.sendQueue1 <- self.constructPacket(typeData, data)
  } else {
    self.comm.sendQueue0 <- self.constructPacket(typeData, data)
  }
  self.dataSent += uint64(len(data))
}

func (self *Session) Signal(sig uint8) {
  self.comm.sendSig <- struct{}{}
  self.comm.sendQueue0 <- self.constructPacket(typeSignal, []byte{sig})
}

func (self *Session) Close() {
  delete(self.comm.Sessions, self.Id)
}
