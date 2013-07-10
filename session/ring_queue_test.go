package session

import (
  "testing"
)

func TestRingQueue(t *testing.T) {
  q := NewRing()
  q.Enqueue(1, []byte("foo"))
  serial, data := q.Dequeue()
  if serial != 1 || string(data) != "foo" { t.Fail() }

  q.Enqueue(1, []byte("foo"))
  q.Enqueue(2, []byte("bar"))
  q.Enqueue(3, []byte("baz"))
  serial, data = q.Dequeue()
  if serial != 1 || string(data) != "foo" { t.Fail() }
  q.Enqueue(4, []byte("qux"))
  serial, data = q.Dequeue()
  if serial != 2 || string(data) != "bar" { t.Fail() }
  serial, data = q.Dequeue()
  if serial != 3 || string(data) != "baz" { t.Fail() }
  serial, data = q.Dequeue()
  if serial != 4 || string(data) != "qux" { t.Fail() }
  serial, data = q.Dequeue()
  if serial != 0 || data != nil { t.Fail() }
}
