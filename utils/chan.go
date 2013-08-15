package utils

import (
  "runtime"
)

type Chan struct {
  In chan interface{}
  Out chan interface{}
}

func NewChan() *Chan {
  c := &Chan{
    In: make(chan interface{}),
    Out: make(chan interface{}),
  }
  ended := make(chan struct{})
  end := make(chan struct{})
  go func(in, out chan interface{}) {
    headNode := new(element)
    headNode.next = headNode
    head := headNode
    tail := headNode
    defer close(ended)
    for {
      if tail != head {
        select {
        case out <- tail.value:
          tail = tail.next // dequeue
        case v := <-in:
          e := &element{value: v}
          if head == tail {
            tail = e
          }
          e.next = head
          head.next.next = e
          head.next = e
        case <-end:
          return
        }
      } else {
        select {
        case v := <-in:
          e := &element{value: v}
          if head == tail {
            tail = e
          }
          e.next = head
          head.next.next = e
          head.next = e
        case <-end:
          return
        }
      }
    }
  }(c.In, c.Out)
  runtime.SetFinalizer(c, func(x *Chan) {
    end <- struct{}{}
    <-ended
    close(x.In)
    close(x.Out)
  })
  return c
}

type element struct {
  value interface{}
  next *element
}
