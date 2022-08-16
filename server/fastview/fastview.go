// fastview implements a builder pattern to implement simple views:
// given an input data format, apply a transformation to a view-model,
// and then  multiplex that data to one or more views.
package fastview

import (
	"context"
	"errors"
	"html/template"

	channerics "github.com/niceyeti/channerics/channels"
)

// TODO: move models around, reorg

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

// ViewComponent implements server side views: Write to allow writing their initial form
// to an output stream and Updates to obtain the chan by which ele-updates are notified.
type ViewComponent interface {
	Updates() <-chan []EleUpdate
	// Parse parses the view-component and adds it to the passed parent template, thus inheriting
	// or possibly extending its definition (func-map, etc). This allows recursively definition
	// view-components. Not sure this is the best design, but worked a posteriori.
	// TODO: a code-smell with this is that the child may rely on the parent's func-map definition;
	// it can extend that definition, or assume it exists and fail when Execute() is called. Depending
	// on something global/non-local is poor composition.
	Parse(*template.Template) (string, error)
}

type ViewBuilder[DataModel any, ViewModel any] struct {
	source      <-chan DataModel // The source type of data, e.g. [][]State
	viewModelFn func(DataModel) ViewModel
	builderFns  []func(<-chan struct{}, <-chan ViewModel) ViewComponent // The set of functions for building views.
	done        <-chan struct{}                                         // Okay if nil
}

func NewViewBuilder[DataModel any, ViewModel any](
	input <-chan DataModel,
) *ViewBuilder[DataModel, ViewModel] {
	return &ViewBuilder[DataModel, ViewModel]{
		source: input,
	}
}

// TODO: add a context function. `func (vb *ViewBuilder) WithContext()``

// TODO: could use builder-pivoting to enforce the order in which builders are called. See Dmitri Nesteruk example.
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
	builderFn func(<-chan struct{}, <-chan ViewModel) ViewComponent,
) *ViewBuilder[DataModel, ViewModel] {
	vb.builderFns = append(vb.builderFns, builderFn)
	return vb
}

// WithContext ensures that all downstream channels are closed when context is cancelled.
// TODO: channel closure communication needs to be evaluated.
func (vb *ViewBuilder[DataModel, ViewModel]) WithContext(
	ctx context.Context,
) *ViewBuilder[DataModel, ViewModel] {
	vb.done = ctx.Done()
	return vb
}

// ErrNoViews is returned when Build() is called before the caller has added any views.
var ErrNoViews error = errors.New("no views to build: WithView must be called")

// ErrNoModel is returned when Build() is called before  WithModel() has been called.
var ErrNoModel error = errors.New("no model specified: WithModel must be called")

// Build executes the stored builders, connecting all of the channels together and returning
// a single aggregated ele-update channel and all the views.
func (vb *ViewBuilder[DataModel, ViewModel]) Build() (views []ViewComponent, err error) {
	if len(vb.builderFns) == 0 {
		return nil, ErrNoViews
	}
	if vb.viewModelFn == nil {
		return nil, ErrNoModel
	}

	// Also consider renaming Adapter to Convert or something...
	vmChan := channerics.Convert(vb.done, vb.source, vb.viewModelFn)
	vmChans := channerics.Broadcast(vb.done, vmChan, len(vb.builderFns))
	for i, build := range vb.builderFns {
		views = append(views, build(vb.done, vmChans[i]))
	}
	return
}
