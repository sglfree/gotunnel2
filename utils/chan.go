package utils

import (
  "log"
  "reflect"
)

func NewChan(in, out interface{}) {
  inValue := reflect.ValueOf(in)
  outValue := reflect.ValueOf(out)
  if inValue.Kind() != reflect.Chan || outValue.Kind() != reflect.Chan {
    log.Fatal("NewChan: argument is not a chan")
  }
  go func() {
    defer outValue.Close()
    headNode := new(element)
    headNode.next = headNode
    head := headNode
    tail := headNode
    for {
      if tail != head {
        chosen, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
          Dir: reflect.SelectSend,
          Chan: outValue,
          Send: tail.value,
        }, reflect.SelectCase{
          Dir: reflect.SelectRecv,
          Chan: inValue,
        }})
        if chosen == 0 {
          tail = tail.next
        } else {
          if !ok { return }
          e := &element{value: v}
          if head == tail {
            tail = e
          }
          e.next = head
          head.next.next = e
          head.next = e
        }
      } else {
        _, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
          Dir: reflect.SelectRecv,
          Chan: inValue,
        }})
        if !ok { return }
        e := &element{value: v}
        if head == tail {
          tail = e
        }
        e.next = head
        head.next.next = e
        head.next = e
      }
    }
  }()
}

type element struct {
  value reflect.Value
  next *element
}
