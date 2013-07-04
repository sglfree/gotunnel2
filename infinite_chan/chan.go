package infinite_chan

import (
  "container/list"
)

type InfiniteChan struct {
  In chan interface{}
  Out chan interface{}
  buffer *list.List
  stop chan struct{}
}

func New() *InfiniteChan {
  c := &InfiniteChan{
    In: make(chan interface{}),
    Out: make(chan interface{}),
    buffer: list.New(),
    stop: make(chan struct{}),
  }
  go c.start()
  return c
}

func (self *InfiniteChan) start() {
  for {
    if self.buffer.Len() > 0 {
      elem := self.buffer.Back()
      value := elem.Value
      select {
      case self.Out <- value:
        self.buffer.Remove(elem)
      case value := <-self.In:
        self.buffer.PushFront(value)
      case <-self.stop:
        return
      }
    } else {
      select {
      case value := <-self.In:
        self.buffer.PushFront(value)
      case <-self.stop:
        return
      }
    }
  }
}

func (self *InfiniteChan) Close() {
  close(self.stop)
  for {
    select {
    case <-self.In:
      continue
    default:
      return
    }
  }
}
