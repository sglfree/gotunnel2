package session

type Queue struct {
  head *Packet
  tail *Packet
}

func NewQueue() *Queue {
  headNode := new(Packet)
  headNode.next = headNode
  return &Queue{
    head: headNode,
    tail: headNode,
  }
}

func (self *Queue) En(packet *Packet) {
  if self.head == self.tail {
    self.tail = packet
  }
  packet.next = self.head
  self.head.next.next = packet
  self.head.next = packet
}

func (self *Queue) De() (ret *Packet) {
  if self.head == self.tail {
    return nil
  }
  ret = self.tail
  self.tail = self.tail.next
  return
}
