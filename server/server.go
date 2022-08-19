package server

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"tabular/models"
	"tabular/server/cell_views"
	"tabular/server/fastview"
	"tabular/server/root_view"

	"github.com/gorilla/websocket"
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
	lastUpdate [][]cell_views.Cell
	rootView   *root_view.RootView
}

// NewServer initializes all of the views and returns a server.
func NewServer(
	ctx context.Context,
	addr string,
	initialStates [][][][]models.State,
	stateUpdates <-chan [][][][]models.State,
) (*Server, error) {
	var err error
	//var tname string
	t := template.New("index")
	rootView := root_view.NewRootView(ctx, initialStates, stateUpdates)
	if _, err = rootView.Parse(t); err != nil {
		return nil, err
	}

	// TODO: this is incomplete/confused abstraction of the views. The last bit of coupling is that
	// the cells must be passed into the template; the template seems to reside at a higher level
	// (the server) which shouldn't know about Cells. Fine for now, but solving this would lead
	// to cleaner/better design. Perhaps the entire index.html generation is a responsibility of
	// the cell_views package. Basically I have arrived at a mixed level of abstraction, whereby
	// views nearly-fully encapsulate information, but not quite, and should continue toward a
	// fully view-agnostic server whose only responsibility is serving.
	initialCells := cell_views.Convert(initialStates)

	return &Server{
		addr:       addr,
		lastUpdate: initialCells,
		rootView:   rootView,
	}, nil
}

func (server *Server) Serve() (err error) {
	http.HandleFunc("/", server.serveIndex)
	http.HandleFunc("/ws", server.serveWebsocket)
	//http.HandleFunc("/profile", pprof.Profile)

	if err = http.ListenAndServe(server.addr, nil); err != nil {
		err = fmt.Errorf("serve: %w", err)
	}

	return
}

// serveWebsocket publishes state updates to the client via websocket.
// TODO: managing multiple websockets, when multiple pages open, etc. These scenarios.
// This currently assumes this handler is hit only once, one client.
// TODO: handle closure and failure paths for websocket.
func (server *Server) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("upgrade:", err)
		return
	}

	defer server.closeWebsocket(ws)
	server.publishUpdates(ws)
}

// TODO: this code is now fubar until I refactor the server and fastviews. This code
// does not define the relationships between clients and websockets, nor closure.
// publishUpdates transforms state updates from the training method into
// view updates sent to the client. "How can I test this" guides the decomposition of
// components.
// Note that taking too long here could block senders on the
// state chan; this will surely change as code develops, be mindful of upstream effects.
func (server *Server) publishUpdates(ws *websocket.Conn) {
	publish := func(updates []fastview.EleUpdate) <-chan error {
		errs := make(chan error)
		go func() {
			defer close(errs)
			if err := ws.WriteJSON(updates); err != nil {
				errs <- err
			}
		}()
		return errs
	}

	// Watch for state updates and push them to the client.
	// Updates are published per a max number of updates per second.
	last := time.Now()
	resolution := time.Millisecond * 200
	var done <-chan error
	for updates := range server.rootView.Updates() {
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
func (server *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html")

	_ = server.rootView.Template().Execute(os.Stdout, server.lastUpdate)

	// FUTURE: see note elsewhere. Execute requires the initial State or Cell data, but the server
	// shouldn't know about either type, hence this should be moved down...
	if err := server.rootView.Template().Execute(w, server.lastUpdate); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
}
