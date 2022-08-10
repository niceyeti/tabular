/*
ViewComponent implements a builder pattern to implement simple views:
given an input data format, apply a transformation to a view-model,
and then  multiplex that data to one or more views.
*/
package fastview

import (
	"context"
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
	Views   []*View            // The set of views (implementing )
	Updates <-chan []EleUpdate // All of the View ele-update chans fanned into a single channel of values to send to client
}

type ViewBuilder[DataModel any, ViewModel any] struct {
	source      <-chan DataModel // The source type of data, e.g. [][]State
	viewModelFn func(DataModel) ViewModel
	done        <-chan struct{}                // Okay if nil
	builderFns  []func(<-chan ViewModel) *View // The set of functions for building views.
	//updates     chan []EleUpdate               // All of the View ele-update chans fanned into a single channel of values to send to client
}

func NewViewBuilder[DataModel any, ViewModel any](
	input <-chan DataModel,
) *ViewBuilder[DataModel, ViewModel] {
	return &ViewBuilder[DataModel, ViewModel]{
		source: input,
	}
}

// TODO: add a context function. `func (vb *ViewBuilder) WithContext()``

// TODO: could use pivoting to enforce the order in which builders are called.
// WithModel creates a new channel derived from the passed function to convert
// items to the target view-model data type.
func (vb *ViewBuilder[DataModel, ViewModel]) WithModel(
	convert func(DataModel) ViewModel,
) *ViewBuilder[DataModel, ViewModel] {
	vb.viewModelFn = convert
	return vb
}

// WithView adds a view to the list of views to build. They will be returned in the same
// order as built when Build() is called.
func (vb *ViewBuilder[DataModel, ViewModel]) WithView(
	builderFn func(<-chan ViewModel) *View,
) *ViewBuilder[DataModel, ViewModel] {
	vb.builderFns = append(vb.builderFns, builderFn)
	return vb
}

// WithContext ensures that all downstream channels are closed when context is cancelled.
// TODO: channel closure communication needs to be evaluated.
func (vb *ViewBuilder[DataModel, ViewModel]) WithContext(
	ctx context.Context,
) {
	vb.done = ctx.Done()
}

// ErrNoViews is returned when Build() is called before the caller has added any views.
var ErrNoViews error = errors.New("no views to build: WithView must be called")

// ErrNoModel is returned when Build() is called before  WithModel() has been called.
var ErrNoModel error = errors.New("no model specified: WithModel must be called")

// Build executes the stored builders, connecting all of the channels together and returning
// a single aggregated ele-update channel and all the views.
func (vb *ViewBuilder[DataModel, ViewModel]) Build() (*ViewComponent, error) {
	if len(vb.builderFns) == 0 {
		return nil, ErrNoViews
	}
	if vb.viewModelFn == nil {
		return nil, ErrNoModel
	}

	// Setup the view-model channels to broadcast data to all views
	var vmChan chan ViewModel = make(chan ViewModel)
	go func() {
		for item := range channerics.OrDone[DataModel](vb.done, vb.source) {
			select {
			case <-vb.done:
				return
			case vmChan <- vb.viewModelFn(item):
			}

			// done-guard
			select {
			case <-vb.done:
				return
			default:
			}
		}
	}()

	var vmChans []<-chan ViewModel = vb.broadcast(vb.source, len(vb.builderFns), vb.done)
	var views []*View
	var updates []<-chan []EleUpdate
	for i, builder := range vb.builderFns {
		vmChan := make(chan ViewModel)
		go func() {
			for item := range channerics.OrDone[DataModel](vb.done, vb.source) {
				select {
				case <-done:
					return
				case vb.target <- convert(item):
				}
			}
		}()

		view := builder(vmChan)
		views = append(views, view)
		updates = append(updates, view.Updates)
	}

	return &ViewComponent{
		Views:   views,
		Updates: channerics.Merge[ViewModel](updates),
	}
}

// broacast returns a slice of channels of size n that repeat the data of the input channel.
// Every item received via input is sent to every output channel. Note that items are not sent
// in parallel to every output chan, only serially one channel at a time.
// TODO: consider moving to channerics; needs evaluation, seems a bit anti-patternish.
func (vb *ViewBuilder[DataModel, ViewModel]) broadcast(
	input <-chan ViewModel,
	n int,
	done <-chan struct{},
) (outputs []<-chan ViewModel) {
	outChans := make([]chan ViewModel, n)
	for i := 0; i < n; i++ {
		outChans[i] = make(chan ViewModel)
		outputs = append(outputs, outChans[i])
	}

	go func() {
		for item := range channerics.OrDone[ViewModel](done, input) {
			for _, vmChan := range outChans {
				select {
				case vmChan <- item:
				case <-done:
					return
				}
			}
		}
	}()

	return
}

//     NewViewBuilder[DataModel, ViewModel](source chan DataModel) *ViewBuilder
//     vb.WithModel(func([]DataModel) []ViewModel)
//     vb.WithView(chan []ViewModel -> NewValuesGridView(t2_chan))
//     vb.Build()  <- execute the builder to get views and ele-update chan; delaying execution of stored funcs allows setting up multiplexing
//						of the @target channel to potentially several view listeners
