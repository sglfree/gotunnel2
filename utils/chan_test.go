package utils

import (
  "testing"
  "fmt"
  "runtime"
)

func TestChan(t *testing.T) {
  for ci := 0; ci < 10; ci++ {
    c := NewChan()
    n := 1000
    var memStats runtime.MemStats
    for i := 0; i < n; i++ {
      c.In <- fmt.Sprintf("%d", i)
    }
    for i := 0; i < n; i++ {
      s := (<-c.Out).(string)
      if s != fmt.Sprintf("%d", i) {
        t.Fail()
      }
      runtime.GC()
      if i % 100 == 0 {
        runtime.ReadMemStats(&memStats)
        fmt.Printf("%v %d\n", memStats.Alloc, runtime.NumGoroutine())
      }
    }
    runtime.GC()
  }
  fmt.Printf("check whether memory or goroutine is leaking.\n")

  c := NewChan()
  select {
  case <-c.Out:
    t.Fail()
  default:
  }
}
