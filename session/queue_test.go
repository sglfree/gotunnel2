package session

import (
  "testing"
  "container/heap"
)

func TestQueue(t *testing.T) {
  q := &Queue{}
  heap.Init(q)
  es := map[int][]byte{
    1: []byte("foo"),
    3: []byte("bar"),
    2: []byte("baz"),
  }
  for priority, b := range es {
    item := &QueueItem{
      data: b,
      priority: priority,
    }
    heap.Push(q, item)
  }
  item := heap.Pop(q).(*QueueItem)
  if string(item.data) != "bar" { t.Fail() }
  item = heap.Pop(q).(*QueueItem)
  if string(item.data) != "baz" { t.Fail() }
  item = heap.Pop(q).(*QueueItem)
  if string(item.data) != "foo" { t.Fail() }
  if q.Len() != 0 { t.Fail() }
}
