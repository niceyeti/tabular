package atomic_float

import (
	"math"
	"sync/atomic"
	"unsafe"
)

// TODO: implement these methods on an AtomicFloat64 type.

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

// Atomically read the float64.
// This definition is needed to ensure that read values are not stale/dirty local copies,
// or equivalently stated that the value is synchronized with main memory.
func AtomicRead(val *float64) (value float64) {
	uint_val := atomic.LoadUint64((*uint64)(unsafe.Pointer(val)))
	return math.Float64frombits(uint_val)
}

// Atomically add to the float64.
// Note: online versions of this repeatedly attempt to add @addend to the float in a for loop
// until the addition succeeds, whether or not the pointee changes in between, which is
// logically incorrect. If the pointee changes while we're operation upon it, it is better
// for the caller to know and take some other action (drop the update, recalculate, etc).
func AtomicAdd(val *float64, addend float64) (new_val float64, succeeded bool) {
	old := AtomicRead(val)
	new_val = old + addend
	succeeded = atomic.CompareAndSwapUint64(
		(*uint64)(unsafe.Pointer(val)),
		math.Float64bits(old),
		math.Float64bits(new_val))
	return
}

// Atomically sets a float64.
func AtomicSet(val *float64, new_val float64) (succeeded bool) {
	old := AtomicRead(val)
	succeeded = atomic.CompareAndSwapUint64(
		(*uint64)(unsafe.Pointer(val)),
		math.Float64bits(old),
		math.Float64bits(new_val))
	return
}
