package root_view

import (
	"context"
	"html/template"
	"log"
	"time"

	"tabular/models"
	"tabular/server/cell_views"
	"tabular/server/fastview"

	channerics "github.com/niceyeti/channerics/channels"
)

// RootView is the main page's index.html, which is the container for all the
// view components, the wiring for their channels, etc.
type RootView struct {
	views   []fastview.ViewComponent
	updates <-chan []fastview.EleUpdate
}

// NewRootView create the main page and the views it contains.
func NewRootView(
	ctx context.Context,
	initialStates [][][][]models.State,
	stateUpdates <-chan [][][][]models.State,
) *RootView {
	// Build all of the views on server construction. This is a tad weird, and has alternatives.
	// For example views could be constructed on the fly per endpoint, broken out by view (separate pages).
	// But this could also be done by building/managing the views in advance and querying them on the fly.
	// So whatevs. I guess its nice that the factory provides this mobile encapsulation of views and chans,
	// and extends other options. Serving views is the server's only responsibility, so this fits.
	views, err := fastview.NewViewBuilder[[][][][]models.State, [][]cell_views.Cell]().
		WithContext(ctx).
		WithModel(stateUpdates, cell_views.Convert).
		WithView(func(
			done <-chan struct{},
			cellUpdates <-chan [][]cell_views.Cell) fastview.ViewComponent {
			return cell_views.NewValuesGrid(done, cellUpdates)
		}).
		WithView(func(
			done <-chan struct{},
			cellUpdates <-chan [][]cell_views.Cell) fastview.ViewComponent {
			return cell_views.NewValueFunction(done, cellUpdates)
		}).
		Build()

	if err != nil {
		log.Fatal(err)
	}

	// TODO: this is a bandaid. Similar to the index-html template note, by abstracting
	// the views I have left the server in a state of insufficient abstraction. The next
	// step will be figuring out where some of this can live appropriately. For example,
	// dependency-inversion suggests that the websocket should be passed into some view-component
	// (a page representing a coherent collection of views), which then fans-in the ele-update
	// channels and throttles its updates to the clients. The primary models here are all fastview,
	// so perhaps this is clearly part of a controller for fastview. Testability drives
	// decomposition.
	updates := fanIn(ctx.Done(), views)

	return &RootView{
		views:   views,
		updates: updates,
	}
}

// Updates returns the main ele-update channel for all the views.
func (rt *RootView) Updates() <-chan []fastview.EleUpdate {
	return rt.updates
}

// Parse builds the main page's template, with websocket bootstrap code, and returns its name.
// It also sets up the func-map that many child components depend on.
func (rv *RootView) Parse(
	parent *template.Template,
) (name string, err error) {
	// Build the func-map, passed recursively to child view components. Note this is a very
	// kludgy pattern, as a view may specify a function call defined above it, or override/add
	// other func definitions. Overall this is just stupid loss of control to fight with; the views
	// should instead add funcs the same way library dependencies are added by calling them. This could
	// be done a number of ways; every component defines all of the funcs it needs, or they get added
	// progressively. The requirement is that components/devs must know when they create a conflict.
	// I don't think this is a hard problem to solve, once one stops approaching it from the confines
	// of satisfying the template package just to 'make things work', as the current solution does.
	rt := parent.Funcs(
		template.FuncMap{
			"add":  func(i, j int) int { return i + j },
			"sub":  func(i, j int) int { return i - j },
			"mult": func(i, j int) int { return i * j },
			"div":  func(i, j int) int { return i / j },
			"max": func(i, j int) int {
				if i > j {
					return i
				}
				return j
			},
		})

	viewTemplates := []string{}
	for _, vc := range rv.views {
		if tname, parseErr := vc.Parse(rt); parseErr != nil {
			err = parseErr
			return
		} else {
			viewTemplates = append(viewTemplates, tname)
		}
	}

	// Specify the nested templates
	var bodySpec string
	for _, tname := range viewTemplates {
		bodySpec += (`{{ template "` + tname + `" . }}`)
	}

	// The main template bootstraps the rest: sets up client websocket and updates, aggregates views.
	name = "mainpage"
	indexTemplate := `
	{{ define "` + name + `" }}
	<!DOCTYPE html>
	<html>
		<head>
			<link rel="icon" href="data:,">
			<!--This is the client bootstrap code by which the server pushes new data to the view via websocket.-->
			<script>
				const ws = new WebSocket("ws://localhost:8080/ws");
				ws.onopen = function (event) {
					console.log("Web socket opened")
				};

				// Listen for errors
				ws.onerror = function (event) {
					console.log('WebSocket error: ', event);
				};

				// The meat: when the server pushes view updates, find these eles and update them.
				ws.onmessage = function (event) {
					items = JSON.parse(event.data)
					// FUTURE: scope the updates per view. Not really needed now, just grab them by id from doc level.
					// Iterate the data updates
					for (const update of items) {
						const ele = document.getElementById(update.EleId)
						for (const op of update.Ops) {
							if (op.Key === "textContent") {
								ele.textContent = op.Value;
							} else {
								ele.setAttribute(op.Key, op.Value)
							}
						}
					}
				}
			</script>
		</head>
		<body>
		` + bodySpec + `
		</body></html>
	{{ end }}
	`

	_, err = rt.Parse(indexTemplate)
	return
}

// fanIn aggregates the views' ele-update channels into a single channel,
// and throttle its output.
// TODO: see note in caller. This is needs a different home
func fanIn(
	done <-chan struct{},
	views []fastview.ViewComponent,
) <-chan []fastview.EleUpdate {
	inputs := make([]<-chan []fastview.EleUpdate, len(views))
	for i, view := range views {
		inputs[i] = view.Updates()
	}
	return batchify(
		done,
		channerics.Merge(done, inputs...),
		time.Millisecond*20)
}

// batchify batches within the passed time frame before sending, over-writing previously
// received values for the same ele-id. This ensures that redundant updates for the
// same ele-id are not sent, and only the latest values are sent.
func batchify(
	done <-chan struct{},
	source <-chan []fastview.EleUpdate,
	rate time.Duration,
) <-chan []fastview.EleUpdate {
	output := make(chan []fastview.EleUpdate)

	go func() {
		defer close(output)

		data := map[string]fastview.EleUpdate{}
		last := time.Now()
		for updates := range channerics.OrDone(done, source) {
			// Intentionally overwrites pre-exisiting values for an ele-id within this batch's time frame.
			for _, update := range updates {
				data[update.EleId] = update
			}

			if time.Since(last) > rate && len(updates) > 0 {
				select {
				case output <- slicedVals(data):
					data = map[string]fastview.EleUpdate{}
					last = time.Now()
				case <-done:
					return
				}
			}
		}
	}()

	return output
}

// returns the values of a map as a slice
func slicedVals[T1 comparable, T2 any](mp map[T1]T2) (sliced []T2) {
	for _, v := range mp {
		sliced = append(sliced, v)
	}
	return
}
