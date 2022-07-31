package atomic_helpers

import (
	"math"
	"sync/atomic"
	"unsafe"
)

// Atomically read a float64.
func AtomicRead(val *float64) (value float64) {
	return math.Float64frombits(atomic.LoadUint64((*uint64)(unsafe.Pointer(val))))
}

// Atomically add to a float64.
func AtomicAdd(val *float64, addend float64) (new_val float64) {
	for {
		old := *val
		new_val = old + addend
		if atomic.CompareAndSwapUint64(
			(*uint64)(unsafe.Pointer(val)),
			math.Float64bits(old),
			math.Float64bits(new_val),
		) {
			break
		}
	}
	return
}

// Atomically sets a float64.
func AtomicSet(val *float64, new_val float64) {
	for {
		old := *val
		if atomic.CompareAndSwapUint64(
			(*uint64)(unsafe.Pointer(val)),
			math.Float64bits(old),
			math.Float64bits(new_val),
		) {
			break
		}
	}
}
