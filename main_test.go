package main

import (
	"sync"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAtomicAdd(t *testing.T) {

	Convey("When atomicAdd is called", t, func() {
		Convey("When multiple writers add to the float value concurrently", func() {
			f64 := float64(0.0)
			num_ops := 3000
			num_writers := 2

			wg := sync.WaitGroup{}
			wg.Add(num_writers)
			adder := func() {
				for i := 0; i < num_ops; i++ {
					atomicAdd(&f64, 1.0)
				}
				wg.Done()
			}

			for i := 0; i < num_writers; i++ {
				go adder()
			}

			wg.Wait()
			So(f64, ShouldEqual, float64(num_ops*num_writers))
		})
	})

}
