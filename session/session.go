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
  self.comm.sendQueue <- self.constructPacket(typeData, data)
}

func (self *Session) Signal(sig uint8) {
  self.comm.sendQueue <- self.constructPacket(typeSignal, []byte{sig})
}

func (self *Session) Close() {
  delete(self.comm.Sessions, self.Id)
}
