package session

import (
  "encoding/binary"
  "bytes"
)

type Session struct {
  Id int64
  comm *Comm
  Obj interface{}
}

func (self *Session) constructPacket(t uint8, data []byte) []byte {
  buf := new(bytes.Buffer)
  binary.Write(buf, binary.LittleEndian, self.comm.nextSerial())
  binary.Write(buf, binary.LittleEndian, self.Id)
  binary.Write(buf, binary.LittleEndian, t)
  binary.Write(buf, binary.LittleEndian, uint32(len(data)))
  buf.Write(data)
  return buf.Bytes()
}

func (self *Session) Send(data []byte) {
  self.comm.sendQueue <- self.constructPacket(typeData, data)
}

func (self *Session) Signal(sig uint8) {
  self.comm.sendQueue <- self.constructPacket(typeSignal, []byte{sig})
}

func (self *Session) Close() {
  delete(self.comm.sessions, self.Id)
}
