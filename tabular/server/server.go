package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"tabular/grid_world"
	"tabular/server/cell_views"
	"tabular/server/fastview"
	"tabular/server/root_view"
)

// Main TODOs:
// 1) websocket pingpong
// 2) Uncle Bob app rearchitecting

// Server serves a single page, to a single client, over a single websocket.
// So intentionally very little generalization, this is just a prototype. This is
// currently useful for solo RL development, just to develop and see html views; but it
// is completely incomplete as a real webserver, as the ele-update channel can be
// listened to by only a single client, among similar quantification issues. You
// could go hog-wild and fully abstract each endpoint (a page and websocket combo),
// beginning with simply muxing the ele-update channel to service multiple clients.
// The server currently builds and represents a single view; no layering at all.
// For experience it would be desirable to rearchitect the server into appropriate
// layers via Uncle Bob's architecture  manifesto. Currently it is a mishmash of
// network, websockets, views, etc., intentionally monolithic because I just wanted
// some views.
//
// Lessons learned: the requirement of serving a basic realtime visualization
// is satisfied by server side events (SSE), and has promising self-contained
// security considerations (runs entirely over http, may not consume as many
// connections, etc.). However I'm going with full-duplex websockets for a more
// expressive language to meet future requirements. The differences are not
// that significant, since this app only requires a small portion of websocket
// functionality at half-duplex. Summary: SSEs are great and modest, suitable
// to something like ads. But websockets are more expressive but connection heavy.
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
	initialStates [][][][]grid_world.State,
	stateUpdates <-chan [][][][]grid_world.State,
) (*Server, error) {
	rootView := root_view.NewRootView(ctx, initialStates, stateUpdates)

	// TODO: this is incomplete/confused abstraction of the views. The last bit of coupling is that
	// the cells must be passed into the template; the template seems to reside at a higher level
	// (the server) which shouldn't know about Cells. Fine for now, but solving this would lead
	// to cleaner/better design. Perhaps the entire index.html generation is a responsibility of
	// the cell_views package. Basically I have arrived at a mixed level of abstraction, whereby
	// views nearly-fully encapsulate information, but not quite, and should continue toward a
	// fully view-agnostic server whose only responsibility is serving. This would be worthwhile
	// golang MVC server research. Best to read Uncle Bob's architecture manifesto and redo the
	// whole app.
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

// NOTE: the websocket code is fubar until/if I refactor the server and fastviews. This code
// does not strictly define the relationships between clients and websockets, nor closure.
// serveWebsocket publishes state updates to the client via websocket.
// TODO: managing multiple websockets, when multiple pages open, etc. These scenarios.
// This currently assumes this handler is hit only once, one client.
// TODO: handle closure and failure paths for websocket.
func (server *Server) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	// FWIW, there is a DDOS risk here by not limiting the number of websocket and http->websocket upgrade attempts per client.
	client, err := fastview.NewClient(server.rootView.Updates(), w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	if err := client.Sync(); err != nil {
		log.Println("websocket endpoint:", err)
		return
	}
}

// Serve the index.html main page.
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

	// FUTURE: see note elsewhere. Execute requires the initial State or Cell data, but the server
	// shouldn't know about either type, hence this should be moved down...
	if err := renderTemplate(w, server.rootView, server.lastUpdate); err != nil {
		_, _ = w.Write([]byte(err.Error()))
	}
}

func renderTemplate(
	w io.Writer,
	vc fastview.ViewComponent,
	data interface{},
) (err error) {
	t := template.New("index.html")
	var tname string
	if tname, err = vc.Parse(t); err != nil {
		return
	}
	if _, err = t.Parse(`{{ template "` + tname + `" . }}`); err != nil {
		return
	}

	err = t.Execute(w, data)
	return
}
