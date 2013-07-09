package session

type RingQueue struct {
  head *Packet
  tail *Packet
}

func NewRing() *RingQueue {
  p := new(Packet)
  p.next = p
  return &RingQueue{
    head: p,
    tail: p,
  }
}

func (self *RingQueue) Enqueue(serial uint64, data []byte) {
  self.head.serial = serial
  self.head.data = data
  if self.head.next == self.tail { // need to insert new node
    p := new(Packet)
    p.next = self.tail
    self.head.next = p
    self.head = p
  } else {
    self.head = self.head.next
  }
}

func (self *RingQueue) Dequeue() *Packet {
  if self.tail == self.head { return nil }
  q := self.tail
  self.tail = self.tail.next
  return q
}

func (self *RingQueue) Peek() *Packet {
  if self.tail == self.head { return nil }
  return self.tail
}
