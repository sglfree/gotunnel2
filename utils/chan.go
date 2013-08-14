package utils

import (
  "runtime"
)

type Chan struct {
  In chan interface{}
  Out chan interface{}
  buffer *queue
  end chan struct{}
}

func NewChan() *Chan {
  c := &Chan{
    In: make(chan interface{}),
    Out: make(chan interface{}),
    buffer: newQueue(),
    end: make(chan struct{}),
  }
  go func(in, out chan interface{}, buffer *queue, end chan struct{}) {
    for {
      if buffer.tail != buffer.head {
        select {
        case out <- buffer.tail.value:
          buffer.tail = buffer.tail.next // dequeue
        case e := <-in:
          buffer.En(e)
        case <-end:
          return
        }
      } else {
        select {
        case e := <-in:
          buffer.En(e)
        case <-end:
          return
        }
      }
    }
  }(c.In, c.Out, c.buffer, c.end)
  runtime.SetFinalizer(c, func(x *Chan) {
    x.end <- struct{}{}
  })
  return c
}

type element struct {
  value interface{}
  next *element
}

type queue struct {
  head *element
  tail *element
}

func newQueue() *queue {
  headNode := new(element)
  headNode.next = headNode
  return &queue{
    head: headNode,
    tail: headNode,
  }
}

func (self *queue) En(v interface{}) {
  e := &element{value: v}
  if self.head == self.tail {
    self.tail = e
  }
  e.next = self.head
  self.head.next.next = e
  self.head.next = e
}
