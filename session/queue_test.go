package session

import (
  "testing"
)

func TestQueue(t *testing.T) {
  q := NewQueue()
  q.En(&Packet{serial: 1})
  q.En(&Packet{serial: 2})
  q.En(&Packet{serial: 3})
  q.En(&Packet{serial: 4})
  p := q.De()
  if p.serial != 1 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 2 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 3 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 4 {
    t.Fail()
  }
  p = q.De()
  if p != nil {
    t.Fail()
  }
  q.En(&Packet{serial: 1})
  q.En(&Packet{serial: 2})
  q.En(&Packet{serial: 3})
  q.En(&Packet{serial: 4})
  p = q.De()
  if p.serial != 1 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 2 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 3 {
    t.Fail()
  }
  p = q.De()
  if p.serial != 4 {
    t.Fail()
  }
  p = q.De()
  if p != nil {
    t.Fail()
  }
}
