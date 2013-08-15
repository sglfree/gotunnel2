package utils

import (
  "testing"
  "fmt"
  "runtime"
)

func TestChan(t *testing.T) {
  for ci := 0; ci < 10; ci++ {
    in := make(chan string)
    out := make(chan string)
    NewChan(in, out)
    n := 1000
    var memStats runtime.MemStats
    for i := 0; i < n; i++ {
      in <- fmt.Sprintf("%d", i)
    }
    for i := 0; i < n; i++ {
      s := <-out
      if s != fmt.Sprintf("%d", i) {
        t.Fail()
      }
      runtime.GC()
      if i % 100 == 0 {
        runtime.ReadMemStats(&memStats)
        fmt.Printf("%v %d\n", memStats.Alloc, runtime.NumGoroutine())
      }
    }
    close(in)
    runtime.GC()
    _, ok := <-out
    if ok { t.Fail() }
  }
  fmt.Printf("check whether memory or goroutine is leaking.\n")
}
