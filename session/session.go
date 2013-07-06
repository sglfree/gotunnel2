package session

import (
  "encoding/binary"
  "bytes"
  "time"
)

const SESSION_CLEANNING_TIMEOUT = 60

type Session struct {
  Id int64
  comm *Comm
  Obj interface{}
}

func (self *Session) constructPacket(t uint8, data []byte) []byte {
  buf := new(bytes.Buffer)
  binary.Write(buf, binary.LittleEndian, self.Id)
  binary.Write(buf, binary.LittleEndian, t)
  binary.Write(buf, binary.LittleEndian, uint32(len(data)))
  buf.Write(data)
  return buf.Bytes()
}

func (self *Session) Send(data []byte) {
  self.comm.sendQueue.In <- self.constructPacket(typeData, data)
}

func (self *Session) Close() {
  self.comm.sendQueue.In <- self.constructPacket(typeClose, []byte{})
  go func() {
    <-time.After(time.Second * SESSION_CLEANNING_TIMEOUT)
    delete(self.comm.sessions, self.Id)
  }()
}
