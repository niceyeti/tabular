package fastview

import (
	"context"
	"errors"

	channerics "github.com/niceyeti/channerics/channels"
)

// ViewBuilder is a pattern for constructing one or more views that use a common view-model.
// The main responsibility for ViewBuiler is Build(): building views and wiring up chans/context.
type ViewBuilder[DataModel any, ViewModel any] struct {
	source      <-chan DataModel                                        // The source type of data, e.g. [][]State
	viewModelFn func(DataModel) ViewModel                               // Converts input data models to view models.
	builderFns  []func(<-chan struct{}, <-chan ViewModel) ViewComponent // The set of functions for building views.
	done        <-chan struct{}                                         // Okay if nil
}

// NewViewBuilder returns a builder for a given data-model and view-model.
func NewViewBuilder[DataModel any, ViewModel any]() *ViewBuilder[DataModel, ViewModel] {
	return &ViewBuilder[DataModel, ViewModel]{}
}

// TODO: could use builder-pivoting to enforce the order in which builders are called. See Dmitri Nesteruk example.
// WithModel creates a new channel derived from the passed function to convert
// items to the target view-model data type.
func (vb *ViewBuilder[DataModel, ViewModel]) WithModel(
	input <-chan DataModel,
	convert func(DataModel) ViewModel,
) *ViewBuilder[DataModel, ViewModel] {
	vb.source = input
	vb.viewModelFn = convert
	return vb
}

// ViewBuilderFunc builds a view from an input view-model channel and a 'done' channel for cleanup.
type ViewBuilderFunc[ViewModel any] func(<-chan struct{}, <-chan ViewModel) ViewComponent

// WithView adds a view to the list of views to build.
// They are returned in the same order as built when Build() is called.
func (vb *ViewBuilder[DataModel, ViewModel]) WithView(
	builderFn ViewBuilderFunc[ViewModel],
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

// Build executes the stored builders, connecting the channels together and returning
// a single aggregated ele-update channel and all the views.
func (vb *ViewBuilder[DataModel, ViewModel]) Build() (views []ViewComponent, err error) {
	if len(vb.builderFns) == 0 {
		return nil, ErrNoViews
	}
	if vb.viewModelFn == nil {
		return nil, ErrNoModel
	}

	vmChan := channerics.Convert(vb.done, vb.source, vb.viewModelFn)
	vmChans := channerics.Broadcast(vb.done, vmChan, len(vb.builderFns))
	for i, build := range vb.builderFns {
		views = append(views, build(vb.done, vmChans[i]))
	}
	return
}
