package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"tabular/models"
	"tabular/server/cell_views"
	"tabular/server/fastview"
	"tabular/server/root_view"

	"github.com/gorilla/websocket"
)

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
	initialStates [][][][]models.State,
	stateUpdates <-chan [][][][]models.State,
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
	resolution := time.Millisecond * 100
	var done <-chan error
	for updates := range server.rootView.Updates() {
		// Drop updates when receiving too quickly.
		if time.Since(last) < resolution {
			continue
		}

		// Await pending publication before publishing a subsequent one.
		if done != nil {
			select {
			case err, isErr := <-done:
				if isErr {
					fmt.Printf("%T\n%v\n", err, err)
					return
				}
			default:
				continue
			}
		}

		last = time.Now()
		if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			fmt.Printf("%T\n%v\n", err, err)
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
