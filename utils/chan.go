package utils

import (
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
		buffer := make([]interface{}, 0, 32)
		for {
			if len(buffer) > 0 {
				chosen, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
					Dir:  reflect.SelectSend,
					Chan: outValue,
					Send: buffer[0].(reflect.Value),
				}, reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: inValue,
				}})
				if chosen == 0 { // out
					buffer = buffer[1:]
				} else {
					if !ok {
						return
					}
					buffer = append(buffer, v)
				}
			} else {
				_, v, ok := reflect.Select([]reflect.SelectCase{reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: inValue,
				}})
				if !ok {
					return
				}
				buffer = append(buffer, v)
			}
		}
	}()
	return
}
