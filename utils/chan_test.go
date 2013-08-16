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
    select {
    case <-out: t.Fail()
    default:
    }
    runtime.GC()
  }
  fmt.Printf("check whether memory or goroutine is leaking.\n")

  in := make(chan int)
  out := make(chan int)
  NewChan(in, out)
  select {
  case <-out:
    t.Fail()
  default:
  }
}

func BenchmarkChan(b *testing.B) {
  in := make(chan int)
  out := make(chan int)
  NewChan(in, out)
  b.ResetTimer()
  for i := 0; i < b.N; i++ {
    in <- 5
    <-out
  }
}
