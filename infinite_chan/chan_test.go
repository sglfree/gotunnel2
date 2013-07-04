package infinite_chan

import (
  "testing"
)

func TestChan(t *testing.T) {
  c := New()
  n := 100000
  for i := 0; i < n; i++ {
    c.In <- i
  }
  for i := 0; i < n; i++ {
    x := (<-c.Out).(int)
    if i != x {
      t.Fail()
    }
  }
}

func BenchmarkChan (b *testing.B) {
  c := New()
  b.ResetTimer()
  for i := 0; i < b.N; i++ {
    c.In <- true
  }
}
