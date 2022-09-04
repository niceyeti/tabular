package atomic_float

import (
	"math"
	"sync/atomic"
	"unsafe"
)

// Notes:
// - consider gc side effects
// - consider race conditions
// This code 'checks out' despite the code-smell of using the unsafe package.
// But beware the tight guidelines, and minimize critical regions and pointers.
// For example, no unsafe pointer should be stored for more than a few lines of context,
// since the gc may move the original variable around, such that the original pointer
// no longer refers to the variable's location:
// 	tmp := unintptr(unsafe.Pointer(&x)) + unsafe.Offsetof(x.b)
// In this code the gc may run, see that &x is no longer referenced, move it,
// and thus tmp refers to a stale location.

// AtomicFloat64 encapsulates a float64 for non-locking atomic operations.
// WARNING: THIS CODE NEEDS REVIEW BY A GOLANG EXPERT. DO NOT TRUST THIS CODE FOR PRODUCTION.
// I came up with this to cheat my way out the problem of locking a very large matrix accessed
// by a much smaller number of workers. Implementing an atomic float precludes the need for locks.
// However this was only for a personal enrichment project, and has not be thoroughly evaluated,
// it merely 'passes the race detector'.
type AtomicFloat64 struct {
	val float64
}

// NewAtomicFloat64 encapsulates a float64 for atomic operations.
func NewAtomicFloat64(val float64) *AtomicFloat64 {
	return &AtomicFloat64{
		val: val,
	}
}

// Atomically read the float64.
// This definition is needed to ensure that read values are not stale/dirty local copies,
// or equivalently stated that the value is synchronized with main memory.
func (af *AtomicFloat64) AtomicRead() (value float64) {
	uint_val := atomic.LoadUint64((*uint64)(unsafe.Pointer(&af.val)))
	return math.Float64frombits(uint_val)
}

// Atomically add to the float64.
// Note: online versions of this repeatedly attempt to add @addend to the float in a for loop
// until the addition succeeds, whether or not the pointee changes in between, which is
// logically incorrect. If the pointee changes while we're operating upon it, it is better
// for the caller to know and take some other action (drop the update, recalculate, etc).
func (af *AtomicFloat64) AtomicAdd(addend float64) (newVal float64, succeeded bool) {
	old := af.AtomicRead()
	newVal = old + addend
	succeeded = atomic.CompareAndSwapUint64(
		(*uint64)(unsafe.Pointer(&af.val)),
		math.Float64bits(old),
		math.Float64bits(newVal))
	return
}

// AtomicSet sets the float64, returns true on success.
func (af *AtomicFloat64) AtomicSet(new_val float64) (succeeded bool) {
	old := af.AtomicRead()
	succeeded = atomic.CompareAndSwapUint64(
		(*uint64)(unsafe.Pointer(&af.val)),
		math.Float64bits(old),
		math.Float64bits(new_val))
	return
}
