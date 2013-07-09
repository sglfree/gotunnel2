package session

import (
  "testing"
)

func TestRingQueue(t *testing.T) {
  q := NewRing()
  q.Enqueue(1, []byte("foo"))
  p := q.Dequeue()
  if p.serial != 1 || string(p.data) != "foo" { t.Fail() }

  q.Enqueue(1, []byte("foo"))
  q.Enqueue(2, []byte("bar"))
  q.Enqueue(3, []byte("baz"))
  p = q.Dequeue()
  if p.serial != 1 || string(p.data) != "foo" { t.Fail() }
  q.Enqueue(4, []byte("qux"))
  p = q.Dequeue()
  if p.serial != 2 || string(p.data) != "bar" { t.Fail() }
  p = q.Dequeue()
  if p.serial != 3 || string(p.data) != "baz" { t.Fail() }
  p = q.Dequeue()
  if p.serial != 4 || string(p.data) != "qux" { t.Fail() }
  p = q.Dequeue()
  if p != nil { t.Fail() }
}
