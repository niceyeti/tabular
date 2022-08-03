package atomic_float

import (
	"math"
	"sync/atomic"
	"unsafe"
)

/*
Gist:
- consider gc side effects
- consider race conditions
This code 'checks out' despite the code-smell of using the unsafe package.
But beware the tight guidelines, and minimize critical regions and pointers.
For example, no unsafe pointer should be stored for more than a few lines of context,
since the gc may move the original variable around, such that the original pointer
no longer refers to the variable's location:
	tmp := unintptr(unsafe.Pointer(&x)) + unsafe.Offsetof(x.b)
In this code the gc may run, see that &x is no longer referenced, move it,
and thus tmp refers to a stale location.
*/

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
