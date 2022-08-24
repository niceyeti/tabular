package atomic_float

import (
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAtomicAdd(t *testing.T) {
	Convey("When atomicAdd is called", t, func() {
		Convey("When multiple writers add to the float value concurrently", func() {
			f64 := float64(0.0)
			num_ops := 3000
			num_writers := 200

			start := make(chan struct{})
			wg := sync.WaitGroup{}
			wg.Add(num_writers)
			adder := func() {
				<-start
				for i := 0; i < num_ops; i++ {
					for succeeded := false; !succeeded; _, succeeded = AtomicAdd(&f64, 1.0) {
					}
				}
				wg.Done()
			}

			for i := 0; i < num_writers; i++ {
				go adder()
			}

			// Wait for goroutines to begin
			time.Sleep(time.Millisecond * 10)
			close(start)
			wg.Wait()
			So(f64, ShouldEqual, float64(num_ops*num_writers))
		})

		Convey("When multiple writers increment and decrement the float value concurrently", func() {
			f64 := float64(0.0)
			num_ops := 3000
			num_writers := 200

			start := make(chan struct{})
			wg := sync.WaitGroup{}
			wg.Add(num_writers * 2)
			incrementer := func() {
				<-start
				for i := 0; i < num_ops; i++ {
					for succeeded := false; !succeeded; _, succeeded = AtomicAdd(&f64, 1.0) {
					}
				}
				wg.Done()
			}

			decrementer := func() {
				<-start
				for i := 0; i < num_ops; i++ {
					for succeeded := false; !succeeded; _, succeeded = AtomicAdd(&f64, -1.0) {
					}
				}
				wg.Done()
			}

			for i := 0; i < num_writers; i++ {
				go incrementer()
				go decrementer()
			}

			// Wait for goroutines to begin
			time.Sleep(time.Millisecond * 10)
			close(start)
			wg.Wait()
			So(f64, ShouldEqual, float64(0.0))
		})
	})
}
