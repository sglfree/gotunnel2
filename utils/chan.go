package utils

import (
	"container/list"
	"log"
	"reflect"
)

func MakeChan(in interface{}) (ret interface{}) {
	outValue := reflect.MakeChan(reflect.TypeOf(in), 0)
	ret = outValue.Convert(reflect.ChanOf(reflect.RecvDir, reflect.TypeOf(in).Elem())).Interface()
	inValue := reflect.ValueOf(in)
	if inValue.Kind() != reflect.Chan || outValue.Kind() != reflect.Chan {
		log.Fatal("MakeChan: argument is not a chan")
	}
	go func() {
		defer outValue.Close()
		buffer := list.New()
		for {
			if buffer.Len() > 0 {
				chosen, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
					Dir:  reflect.SelectSend,
					Chan: outValue,
					Send: buffer.Front().Value.(reflect.Value),
				}, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: inValue,
				}})
				if chosen == 0 { // out
					buffer.Remove(buffer.Front())
				} else {
					if !ok {
						return
					}
					buffer.PushBack(v)
				}
			} else {
				_, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: inValue,
				}})
				if !ok {
					return
				}
				buffer.PushBack(v)
			}
		}
	}()
	return
}
