package utils

import (
  "log"
  "reflect"
  "container/list"
)

func NewChan(in, out interface{}) {
  inValue := reflect.ValueOf(in)
  outValue := reflect.ValueOf(out)
  if inValue.Kind() != reflect.Chan || outValue.Kind() != reflect.Chan {
    log.Fatal("NewChan: argument is not a chan")
  }
  go func() {
    defer outValue.Close()
    buffer := list.New()
    for {
      if buffer.Len() > 0 {
        chosen, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
          Dir: reflect.SelectSend,
          Chan: outValue,
          Send: buffer.Front().Value.(reflect.Value),
        }, reflect.SelectCase{
          Dir: reflect.SelectRecv,
          Chan: inValue,
        }})
        if chosen == 0 { // out
          buffer.Remove(buffer.Front())
        } else {
          if !ok { return }
          buffer.PushBack(v)
        }
      } else {
        _, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
          Dir: reflect.SelectRecv,
          Chan: inValue,
        }})
        if !ok { return }
        buffer.PushBack(v)
      }
    }
  }()
}

func MakeChan(in interface{}) interface{} {
  out := reflect.MakeChan(reflect.TypeOf(in), 0)
  NewChan(in, out.Interface())
  return out.Convert(reflect.ChanOf(reflect.RecvDir, reflect.TypeOf(in).Elem())).Interface()
}
