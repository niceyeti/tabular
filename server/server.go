package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"tabular/models"
	"tabular/server/cell_views"
	"tabular/server/fastview"

	"github.com/gorilla/websocket"
	channerics "github.com/niceyeti/channerics/channels"
)

// TODO: refactor the whole server. This is pretty awful but fine for prototyping the realtime svg updates.
var upgrader = websocket.Upgrader{}

const (
	// Time allowed to write a message to the peer.
	writeWait = 1 * time.Second
	// Maximum message size allowed from peer.
	maxMessageSize = 8192
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

/*
Gist: I want to serve svg-based views of training information (value functions, policy info, etc).
Svg is nice because it is declarative; real values map directly to attributes (like heatmaps).
The issue is that while I could regenerate such views from an html template periodically, the client
must then refresh the page to see the new view. Instead I want to push info from the server to the client,
which requires web sockets. It also requires some logic and mapping to determine which values to update.
I wish there was a sophisticated way to do this, but my approach is more or less procedural. Hopefully
something more clever will become clear.

The plan: generate an initial svg containing item id's by which to map RL values to displayed values.
This will be a visual grid of the agent's V(s) values, where each cell has some searchable identifier.
When new values occur, the deltas are sent to the client to update via a simple loop in js.

Task 0: serve a page and demonstrate server side push updates to it.
Task 1: bind this info to the agent value function with mathematical transformation (e.g. color mapping or policy vectors)
Task 3: add additional info (golang runtime telemetry, etc), Q(s,a) values

Lessons learned: the requirement of serving a basic realtime visualization is satisfied by server side events (SSE), and has promising
self-contained security considerations (runs entirely over http, may not consume as many connections, etc.). However
I'm going with full-duplex websockets for a more expressive language to meet future requirements. The differences
are not that significant, since this app only requires a small portion of websocket functionality at half-duplex.
Summary: SSEs are great and modest, suitable to something like ads. But websockets are more expressive but connection heavy.
*/

type Server struct {
	addr string
	// TODO: eliminate? 'last' patterns are always a code smell; the initial state should be pumped regardless...
	lastUpdate [][][][]models.State
	views      []fastview.ViewComponent
	eleUpdates <-chan []fastview.EleUpdate
}

// NewServer initializes all of the views and returns a server.
func NewServer(
	ctx context.Context,
	addr string,
	initialStates [][][][]models.State,
	state_updates <-chan [][][][]models.State) *Server {
	// Build all of the views on server construction. This is a tad weird, and has alternatives.
	// For example views could be constructed on the fly per endpoint, broken out by view (separate pages).
	// But this could also be done by building/managing the views in advance and querying them on the fly.
	// So whatevs. I guess its nice that the factory provides this mobile encapsulation of views and chans,
	// and extends other options. Serving views is the server's only responsibility, so this fits.
	views, err := fastview.NewViewBuilder[[][][][]models.State, [][]cell_views.Cell](state_updates).
		WithContext(ctx).
		WithModel(cell_views.Convert).
		WithView(func(
			cellUpdates <-chan [][]cell_views.Cell,
			done <-chan struct{},
		) fastview.ViewComponent {
			return cell_views.NewValuesGrid("valuesgrid", cellUpdates, done)
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

	return &Server{
		addr:       addr,
		lastUpdate: initialStates,
		views:      views,
		eleUpdates: updates,
	}
}

func (server *Server) Serve() {
	http.HandleFunc("/", server.serve_index)
	http.HandleFunc("/ws", server.serve_websocket)
	//http.HandleFunc("/profile", pprof.Profile)
	// TODO: parameterize port, addr per container requirements. The client bootstrap code must also receive
	// the port number to connect to the web socket.
	if err := http.ListenAndServe(server.addr, nil); err != nil {
		fmt.Println(err)
	}
	fmt.Println("Server started!")
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
	return throttle(
		done,
		channerics.Merge(done, inputs...),
		time.Millisecond)
}

// throttle batches and sends values from the input channel at the passed rate.
// This can be useful when multiple channels produce values independently but driven
// by the same event transactions, such as a single upstream data source.
// TODO: consider elevating to channerics; though this could be more generic (batching
// by time, num items, or other criteria).
// NOTE: the poor synchronicity of time-delayed batching seems like a code smell. Evaluate
// if/when refactoring the server. For example note the property that chaining throttles
// together results in additive delays, whereas it would be desirable for a chain to throttle
// only for the max of all their throttles.
func throttle[T any](
	done <-chan struct{},
	source <-chan []T,
	rate time.Duration, // Could @rate, the throttle condition, be a func() bool instead?
) <-chan []T {
	output := make(chan []T)

	go func() {
		defer close(output)
		batch := []T{}
		last := time.Now()
		for data := range channerics.OrDone(done, source) {
			// TODO: this is unbounded growth within a time frame.
			batch = append(batch, data...)
			if time.Since(last) > rate {
				select {
				case output <- batch:
					last = time.Now()
					batch = batch[:0]
				case <-done:
					return
				}
			}
		}
	}()

	return output
}

// serve_websocket publishes state updates to the client via websocket.
// TODO: managing multiple websockets, when multiple pages open, etc. These scenarios.
// This currently assumes this handler is hit only once, one client.
// TODO: handle closure and failure paths for websocket.
func (server *Server) serve_websocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("upgrade:", err)
		return
	}

	defer server.closeWebsocket(ws)
	server.publish_state_updates(ws)
}

// TODO: this code is now fubar until I refactor the server and fastviews. This code
// does not define the relationships between clients and websockets, nor closure.
// publish_state_updates transforms state updates from the training method into
// view updates sent to the client. "How can I test this" guides the decomposition of
// components.
// Note that taking too long here could block senders on the
// state chan; this will surely change as code develops, be mindful of upstream effects.
func (server *Server) publish_state_updates(ws *websocket.Conn) {
	publish := func(updates []fastview.EleUpdate) <-chan error {
		errchan := make(chan error)
		go func() {
			defer close(errchan)
			if err := ws.WriteJSON(updates); err != nil {
				errchan <- err
			}
		}()
		return errchan
	}
	last := time.Now()
	resolution := time.Millisecond * 200
	var done <-chan error

	// Watch for state updates and push them to the client.
	// Updates are published per a max number of updates per second.
	for updates := range server.eleUpdates {
		//fmt.Println("WS server tick")
		// Drop updates when receiving them too quickly.
		if time.Since(last) < resolution {
			continue
		}

		// Await pending publication before publishing a new one.
		if done != nil {
			select {
			case err, isErr := <-done:
				if isErr {
					fmt.Println(err)
					return
				}
			default:
				continue
			}
		}

		last = time.Now()
		if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			fmt.Println(err)
			return
		}

		done = publish(updates)
	}
}

func (server *Server) closeWebsocket(ws *websocket.Conn) {
	_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
	_ = ws.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	ws.Close()
}

// TODO: cleanup template and its ownership
// FUTURE: it would be a fun problem to solve to devise a robust way to serve multiple
// ui subcomponents (value function, policy visual, etc) and assemble them as one.
func (server *Server) serve_index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	fmt.Println("parsing...")
	// Build the template, bind data.
	// TODO: note the child-template dependency on the func map. Also see the note about the
	// recursive relationships of the views/templates. The same seems to apply here, whereby
	// the func-map could be passed down recursively to child components.
	func_map := template.FuncMap{
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
	}
	// TODO: the fundamental problem with the func-map is that a template should own/define the
	// set of functions it intends to use, rather than inheriting them (coupling). Something
	// is weird about passing this down. A shared func map smells like distorted encapsulation,
	// though Funcs() does specify that a template may override func-map entries.
	t := template.New("index-html").Funcs(func_map)

	view_templates := []*template.Template{}
	for _, vc := range server.views {
		vt := template.Must(vc.Template(func_map))
		view_templates = append(view_templates, vt)
		template.Must(t.AddParseTree(vt.Name(), vt.Tree))
	}

	// TODO: isn't there recursive relationship here wrt to writers of the textual part of the templates?
	// So for instance each could pass down an io.Writer to build there portion of the tree, with parents
	// defining the layout of their children to some degree.

	// The main template bootstraps the rest: sets up client websocket and updates, aggregates views.
	main_template := `<!DOCTYPE html>
	<html>
		<head>
			<link rel="icon" href="data:,">
			<!--This is the client bootstrap code by which the server pushes new data to the view via websocket.-->
			{{ $component_name := "value-function-svg" }}
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
					//const values_ele = document.getElementById({{ $component_name }});
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
		`

	for _, vt := range view_templates {
		// Specify the nested template and pass in its params
		main_template += `{{ template "` + vt.Name() + `" . }}`
	}

	main_template += `
		</body>
	</html>
	`

	var err error
	if t, err = t.Parse(main_template); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	// TODO: this is incomplete abstraction of the views. The last bit of coupling is that
	// the cells must be passed into the template; the template seems to reside at a higher level
	// (the server) which doesn't seem like it should know about Cells. Fine for now, but the
	// solving this would lead to cleaner/better design. Perhaps the entire index.html generation
	// is a responsibility of the cell_views package. Basically I have arrived at a mixed level of
	// abstraction, whereby views nearly-fully encapsulate information, but not quite, and should
	// continue toward a fully view-agnostic server whose only responsibility is serving.
	cells := cell_views.Convert(server.lastUpdate)
	if err = t.Execute(w, cells); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
}
