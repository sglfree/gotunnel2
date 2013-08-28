package utils

import (
	"fmt"
	"runtime"
	"testing"
)

func TestChan(t *testing.T) {
	for ci := 0; ci < 10; ci++ {
		in := make(chan string)
		out := MakeChan(in).(<-chan string)
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
			if i%100 == 0 {
				runtime.ReadMemStats(&memStats)
				fmt.Printf("%v %d\n", memStats.Alloc, runtime.NumGoroutine())
			}
		}
		close(in)
		runtime.GC()
		_, ok := <-out
		if ok {
			t.Fail()
		}
	}
	fmt.Printf("check whether memory or goroutine is leaking.\n")

	in := make(chan int)
	out := MakeChan(in).(<-chan int)
	select {
	case <-out:
		t.Fail()
	default:
	}
}

func BenchmarkChan(b *testing.B) {
	in := make(chan int)
	out := MakeChan(in).(<-chan int)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		in <- 5
		<-out
	}
}
