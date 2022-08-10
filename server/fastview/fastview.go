/*
ViewComponent implements a builder pattern to implement simple views:
given an input data format, apply a transformation to a view-model,
and then  multiplex that data to one or more views.
*/
package fastview

import (
	"errors"

	channerics "github.com/niceyeti/channerics/channels"
)

// EleUpdate is an element identifier and a set of operations to apply to its attributes/content.
type EleUpdate struct {
	// The id by which to find the element
	EleId string
	// Op keys are attrib keys or 'textContent', values are the strings to which these are set.
	// Example: ('x','123') means 'set attribute 'x' to 123. 'textContent' is a reserved key:
	// ('textContent','abc') means 'set ele.textContent to abc'.
	Ops []Op
}

// Op is a key and value. For example an html attribute and its new value.
type Op struct {
	Key   string
	Value string
}

// Implements Write(io.Writer) ???
type View struct {
	// TODO
}

type ViewComponent struct {
	views   []*View            // The set of views (implementing )
	updates <-chan []EleUpdate // All of the View ele-update chans fanned into a single channel of values to send to client
}

type ViewModel interface {
	any
}

type DataModel interface {
	any
}

type ViewBuilder[T1 DataModel, T2 ViewModel] struct {
	source  <-chan T1               // The source type of data, e.g. [][]State
	target  chan T2                 // The target data type, e.g. [][]Cell, State -> Cell
	viewFns []func(<-chan T2) *View // The set of functions for building views.
	updates chan []EleUpdate        // All of the View ele-update chans fanned into a single channel of values to send to client
}

func NewViewBuilder[T1 DataModel, T2 ViewModel](input <-chan T1) *ViewBuilder[T1, T2] {
	return &ViewBuilder[T1, T2]{
		source: input,
	}
}

// TODO: could use pivoting to enforce the order in which builders are called.
// WithModel creates a new channel derived from the passed function to convert
// items to the target view-model data type.
func (vb *ViewBuilder[T1, T2]) WithModel(convert func(T1) T2, done <-chan struct{}) *ViewBuilder[T1, T2] {
	vb.target = make(chan T2)
	go func() {
		for item := range channerics.OrDone[T1](done, vb.source) {
			select {
			case <-done:
				return
			case vb.target <- convert(item):
			}
		}
	}()

	return vb
}

// WithView adds a view to the list of views to build. They will be returned in the same
// order as built when Build() is called.
func (vb *ViewBuilder[T1, T2]) WithView(fn func(<-chan T2) *View) *ViewBuilder[T1, T2] {
	vb.viewFns = append(vb.viewFns, fn)
	return vb
}

// ErrNoViews is returned when Build() is called before the caller has added any views.
var ErrNoViews error = errors.New("no views to build: WithView must be called")

// ErrNoModel is returned when Build() is called before  WithModel() has been called.
var ErrNoModel error = errors.New("no model specified: WithModel must be called")

func (vb *ViewBuilder[T1, T2]) Build(done <-chan struct{}) (*ViewComponent, error) {
	if len(vb.viewFns) == 0 {
		return nil, ErrNoViews
	}
	if vb.target == nil {
		return nil, ErrNoModel
	}

	var views []*View
	var updates []<-chan []EleUpdate
	for _, fn := range vb.viewFns {
		view := fn(vb.target)
		views = append(views, view)
		updates = append(updates, view.Updates)
	}

	return &ViewComponent{
		views:   views,
		updates: channerics.Merge[T2](updates),
	}
}

//     NewViewBuilder[T1, T2](source chan T1) *ViewBuilder
//     vb.WithModel(func([]T1) []T2)
//     vb.WithView(chan []T2 -> NewValuesGridView(t2_chan))
//     vb.Build()  <- execute the builder to get views and ele-update chan; delaying execution of stored funcs allows setting up multiplexing
//						of the @target channel to potentially several view listeners
