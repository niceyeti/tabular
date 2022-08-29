# About

FastView is a small ui library for rapid development of realtime, readonly html visualizations.
It uses html/template to spin up views from models and pushes updates to the via websocket.
I needed something minimal but somewhat performant to build up simple, realtime svg-based views of 
functions for reinforcement learning, and cooked up fastview.

## Views

A view consists of a golang html template, a view model, and a channel generating ele-updates by
which the client view is kept up to date. These elements fairly successfully divide view, view-model, and controller:
* template: each view's template is simply a golang html template. The template describes the view in html,
as well as sets up initial ele identifiers, used later to identify eles to update. The template is defined once
based on the initial data structures (e.g. the State matrix), thus the initial layout is permanent and cannot be
changed later. Parent templates (such as a parent
component or the main html page) passes itself into each child component, such that children add themselves
(`parse`) and possibly extend the parent's func template. There are reasons this pattern is not robust,
and was primarily driven by 'make it work' surrendering to golang/template library requirements; these are
noted in the code
* view-model: the view model is the data structure derived from the [][][][]State matrix, via a conversion
function. Every view must define a view model for converting incoming state updates to their own models; usually
these are just simple book-keeping data structures of derived values that can be used immediately for views, such
as svg-ele attributes values. A collection of views may used the same view-model, and can be organized as such.
* ele-update channel: each view receives its view-model (after conversion from source data, e.g. the State matrix), and exposes an ele-update channel via its Updates() function. The view itself implements the conversion from view-models to ele-updates.

## ViewBuilder

ViewBuilder is a component for building one or more views. Its primary responsibility is merely organizing the components of views: context, input channels, conversion to view-models for a specific set of views of that model, etc. It mainly wires together the channels by which views are both updated and cancelled/disassembled via context.

ViewBuilder is the best place to start refactoring the library for future use; it makes some of the downsides most apparent. Resolving these will lead to better intuition about the 'how should this be done' aspects. See Dmitri Nesteruk's Golang Design Pattern builder examples for advanced pivoting techniques; a very tidy builder interface could be used to construct views.

```
     NewViewBuilder[T1, T2](source chan T1) *ViewBuilder
     vb.WithContext(ctx)
     vb.WithModel(func([]T1) []T2)
     vb.WithView(chan []T2 -> NewValuesGridView(t2_chan))
     vb.Build()  <- execute the builder to get views and ele-update chan; delaying execution of stored 
```