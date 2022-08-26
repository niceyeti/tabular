package server

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"tabular/grid_world"
	"tabular/server/cell_views"
	"tabular/server/fastview"
	"tabular/server/root_view"

	"github.com/gorilla/websocket"
	channerics "github.com/niceyeti/channerics/channels"
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

// serveWebsocket publishes state updates to the client via websocket.
// TODO: managing multiple websockets, when multiple pages open, etc. These scenarios.
// This currently assumes this handler is hit only once, one client.
// TODO: handle closure and failure paths for websocket.
func (server *Server) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	// DDOS risk here if server does not track and limit http->websocket upgrade attempts per client
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("upgrade:", err)
		return
	}

	defer server.closeWebsocket(ws)
	server.publishEleUpdates(r.Context(), ws)
}

// TODO: this code is now fubar until I refactor the server and fastviews. This code
// does not define the relationships between clients and websockets, nor closure.
// publishEleUpdates transforms state updates from the training method into
// view updates sent to the client. "How can I test this" guides the decomposition of
// components.
// TODO: the websocket code exemplifies the externalmost layer in Uncle Bobs architecture: net,
// db, drivers, etc. This should be broken out as such.
// Note that taking too long here could block senders on the
// state chan; this will surely change as code develops, be mindful of upstream effects.
func (server *Server) publishEleUpdates(
	ctx context.Context,
	ws *websocket.Conn,
) {
	// Watch for state updates and push them to the client.
	// Updates are published per a max of updates per second.
	last := time.Now()
	pubResolution := time.Millisecond * 100
	pingResolution := time.Millisecond * 500
	pubCtx, cancelPub := context.WithCancel(ctx)
	defer cancelPub()
	pinger := channerics.NewTicker(pubCtx.Done(), pingResolution)
	lastPong := time.Now()

	// Monitor client health/disconnects
	pong := make(chan struct{})
	defer close(pong)
	ws.SetPongHandler(func(appData string) error {
		pong <- struct{}{}
		return nil
	})

	// Calling a read method on the websocket in turn calls handlers (ping, pong, etc).
	// Thus a read method must be called so ping/pong and other control handlers are called.
	// A separate goroutine is required to monitor the blocking read call.
	// A good example of satisfying the lib requirements is in the chat example:
	// https://github.com/gorilla/websocket/blob/af47554f343b4675b30172ac301638d350db34a5/examples/chat/client.go
	go func() {
		for {
			select {
			case <-pubCtx.Done():
				return
			default:
				// Blocks until a message is available, triggering the PongHandler when pongs are read.
				// All errors from Read methods are permanent, hence publication must be cancelled.
				if _, _, err := ws.ReadMessage(); err != nil {
					cancelPub()
					if isClosure(err) {
						return
					}
					fmt.Println("read pump: ", err)
				}
			}
		}
	}()

	for {
		select {
		case <-pubCtx.Done():
			return
		case <-pinger:
			if time.Since(lastPong) > pingResolution*2 {
				fmt.Println("i said one ping only, vasiliy! closing conn")
				return
			}

			if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				if isError(err) {
					fmt.Printf("ping failed: %T %v", err, err)
				}
				return
			}
		case <-pong:
			lastPong = time.Now()
		case updates := <-server.rootView.Updates():
			// Drop updates when receiving too quickly.
			if time.Since(last) < pubResolution {
				break
			}

			last = time.Now()
			if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				fmt.Printf("failed to set deadline: %T %v", err, err)
				return
			}

			if err := ws.WriteJSON(updates); err != nil {
				if isError(err) {
					fmt.Printf("publish failed: %T %v", err, err)
				}
				return
			}
		}
	}
}

func isError(err error) bool {
	return err != nil && websocket.IsUnexpectedCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway)
}

func isClosure(err error) bool {
	return err != nil && websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway)
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
