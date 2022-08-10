## Problem Statement

Some design notes for the chain of responsibility pattern in the server view endpoints.

The gist is that I want to implement views rapidly using html templates derived from the Cell
definition or other derived models. I want to be able to organize these components in a maintainable
and testable fashion. The server's main index.html endpoint then builds a webpage containing
each realtime component.

For background, it currently works like this: the server implements a websocket endpoint at /ws
by which it pushes view updates to the client. This update logic (and the update objects) are defined
in a small chunk of javascript on the main index.html. When called, the main index.html endpoint constructs
this page and adds to it any view components. The server monitors for State changes from the training
method, transforms these to views models, and pushes updates to the client.
Thus any 'view' component on the main page (e.g. a grid of state values, a plot of the value function, etc)
is responsible for its initial layout definiton (as an html/template) and also for converting State updates
to EleUpdate operations to be sent to the client.

## MEGA BONUS 
The above lends a procedural implementation with varying degrees of reuse. But how cool would it be
to define this problem more comprehensively in observable fashion? I envision an end-to-end library that directly
pushes to the client. This is motivated by the view that the client's view is merely a series of transformations
(functions) applied to State updates. It would be awesome to implement a library/framework fully encapsulating
the network, document generating (writers), and model logic in a clean manner.
- This will be accomplished by modeling the current set of functions of the prototype, then applying various patterns.
    * model the problem as a computational graph
    * define the high-level code and how it should look
    * the whole thing might be a computational graph implemented as chained template-methods (.Where() .Tee() .Project() .With() .As()) each
      returning a channel or otherwise linked through channels.
Golang high level problem statements: a library for converting data into realtime views, rapid development.
- I am not a fan of decorators as code, but for the sake of motivating ideas, could field tags support
  the translation of models into svg/html/xml view updates?


## Notes

1) Html composition: 
    Each View consists of an initial template and a definition for appyling EleUpates.

    type View interface {
        // Init writes the view's initial template to the passed writer, and returns a
        // func to call for updates to push to the client.
        Init(io.Writer) func[[][]]Cell T](t T) []EleUpdate
        // Aternative form: encapsulates knowledge of the incoming types it monitors for changes.
        Init(io.Writer) func() []EleUpdate
    }

    Problem: the type T makes this not generic enough. There is an orthogonal set of transformations
    to the data assumed by this interface. 

    Controller:
        data transformations and updates; these must propagate to the views via EleUpdate
        [][]State -> [][]Cell
                        ^
                        Views observe these

    Views:
        initial templates, some update mechanism


    Code:
        grommet view.go:

        NewView(chan grommet) { }
        // Hmm... these don't mix. Writing the page is somewhat independent of the view's model updates
        // in most regards.
        Init(io.Writer) chan []EleUpdate

    Controller:
        cell_views, cell_ele_updates := Model(chan [][]State) // some/any model exposing derivations from the incoming States
            .As(state -> Cell) // convert the data to a target type for these views
            .WithView(cell_chan -> NewValuesGridView(cell_chan))   // build the grid-view
            .WithView(cell_chan -> NewValueFuncView(cell_chan))    // build the value function view
            .Build() //Terminal call: returns views and ele_update chans
        * Thus the cell_views expose Init(io.Writer) to write the initial form of the view, and cell_ele_updates communicates their updates.
          Note how elegantly one could build other views in this form: convert incoming data to the target type
          for the view, pass it to one or more derived views, and build a collection of views/update-chans.

            1) index-endpoint: defines the initial webpage layout with the socket js code
            2) update-worker: monitors for ele-updates and pushes these to the client
                FUTURE: could broadcast the updates to all clients
        
        Pro/Con:
        + All views can be maintained independently
        + All code can be tested independently: both the views and the chan library
        - Potential inflexibility: like obs lib at *** all change is synchronous. New views require code changes and recompiling anew.
        - Obviously only supports read-only views (scientific visualization, etc.)
        + In sum, I think this satisfies a simple practice project like Tabular. No further reqs.
