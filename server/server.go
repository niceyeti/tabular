package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	. "tabular/atomic_helpers"
	. "tabular/models"

	"github.com/gorilla/websocket"
)

// TODO: refactor the whole server. This is pretty awful but fine for prototyping the realtime svg updates.
var upgrader = websocket.Upgrader{}

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second
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

Lessons learned: the requirement of serving a basic realtime visualization is satisfied by SSE, and has promising
self-contained security considerations (runs entirely over http, may not consume as many connections). However
I'm going with full-duplex websockets for a more expressive language to meet future requirements. The differences
are not that significant, since this app only requires a small portion of websocket functionality at half-duplex.
Summary: SSEs are great and modest, suitable to something like ads. But websockets are more expressive but connection heavy.
*/

// Converts the [x][y][vx][vy]State gridworld to a simpler x/y only set of cells,
// oriented in svg coordinate system such that [0][0] is the logical cell that would
// be printed in the console at top left. This purpose of [][]Cells is convenient
// traversal and data for generating golang templates; otherwise one must implement
// ugly template funcs to map the [][][][]State structure to views, which is tedious.
// The purpose of Cell itself is to contain ephemeral descriptors (max action direction,
// etc) useful for putting in the view.
type Cell struct {
	X, Y int
	Max  float64
}

// EleUpdate is an element identifier and a set of operations to apply to its attributes/content.
type EleUpdate struct {
	// The id by which to find the element
	EleId string
	// Op keys are attrib keys or 'textContent', values are the new strings to which these are set.
	// Example: ('x','123') means 'set attribute 'x' to 123. 'textContent' is a reserved key:
	// ('textContent','abc') means 'set ele.textContent to abc'.
	Ops []Op
}

type Op struct {
	Key   string
	Value string
}

func convert_states_to_cells(states [][][][]State) (cells [][]Cell) {
	cells = make([][]Cell, len(states))
	max_y := len(states[0])
	for x := range states {
		cells[x] = make([]Cell, max_y)
	}

	Visit_xy_states(states, func(velstates [][]State) {
		x, y := velstates[0][0].X, velstates[0][0].Y
		// flip the y indices for displaying in svg coordinate system
		cells[x][y] = Cell{
			X: x, Y: max_y - y - 1,
			Max: AtomicRead(&Max_vel_state(velstates).Value),
		}
	})

	return
}

// Returns the set of view updates needed for the view to reflect the current values.
func get_cell_updates(cells [][]Cell) (updates []EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			updates = append(updates, EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-text", cell.X, cell.Y),
				Ops: []Op{
					{"textContent", fmt.Sprintf("%.2f", cell.Max)},
				},
			})
		}
	}
	return
}

type Server struct {
	last_update [][][][]State
	// TODO: refactor to eliminate, replace with chan fed by hook fn training updates
	state_updates <-chan [][][][]State
}

/*
Server: this server is a monolith. A pure server would abstract away the details of each
visual component from some builder/factories for generating them (and their websockets),
and would then simply coordinate them. This server instead has it all: knowledge of
templates, converting models to view models, and bootstrapping web sockets.
*/

// TODO: refactor server to accept State chan for update notifications from training hook
func NewServer(
	initial_states [][][][]State,
	state_updates <-chan [][][][]State) *Server {
	return &Server{
		last_update:   initial_states,
		state_updates: state_updates,
	}
}

func (server *Server) Serve() {
	http.HandleFunc("/", server.serve_main)
	http.HandleFunc("/ws", server.serve_websocket)
	// TODO: parameterize port, addr per container requirements. The client bootstrap code must also receive
	// the port number to connect to the web socket.
	_ = http.ListenAndServe(":8080", nil)
}

func (server *Server) serve_websocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}

	// TODO: this will be refactored to listen for updates from the training hook
	// TODO: determine where closure belongs
	defer server.closeWebsocket(ws)

	// Watch for state updates and push them to the client.
	for states := range server.state_updates {
		fmt.Println("WS server tick")
		_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
		cells := convert_states_to_cells(states)
		updates := get_cell_updates(cells)
		if err := ws.WriteJSON(updates); err != nil {
			panic(err)
			//break
		}
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
func (server *Server) serve_main(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("parsing...")
	// Build the template, bind data
	t := template.New("state-values").Funcs(
		template.FuncMap{
			"add":  func(i, j int) int { return i + j },
			"sub":  func(i, j int) int { return i - j },
			"mult": func(i, j int) int { return i * j },
			"div":  func(i, j int) int { return i / j },
		})

	var err error
	if _, err = t.Parse(`<html>
		<head>
			<!--This is the client bootstrap code by which the server pushes new data and loads it into the view via websocket.-->
			{{ $component_name := "value-function-svg" }}
			<script>
				const ws = new WebSocket("ws://localhost:8080/ws");
				ws.onopen = function (event) {
					console.log("Web socket opened")
				};

				// Listen for errors
				ws.addEventListener('error', function (event) {
					console.log('WebSocket error: ', event);
				});

				// The meat: when the server pushes view updates, find these eles and update them.
				ws.onmessage = function (event) {
					//console.log(event.data);
					console.log("updating ui")
					items = JSON.parse(event.data)
					const values_ele = document.getElementById({{ $component_name }});
					// Iterate the data updates
					for (const update of items) {
						const ele = values_ele.getElementById(update.EleId)
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
		{{ $x_cells := len . }}
		{{ $y_cells := len (index . 0)}}
		{{ $width := 500 }}
		{{ $cell_width := div $width $x_cells }}
		{{ $height := mult $cell_width $y_cells }}
		{{ $cell_height := $cell_width}}
		{{ $half_height := div $cell_height 2 }}
		{{ $half_width := div $cell_width 2 }}
		<div>Num cells: {{ $x_cells }} Y cells: {{ $y_cells}}</div>
			<div id="state_values">
				<svg id="{{ $component_name }}" width="{{ $width }}px" height="{{ $height }}px">
				{{ range $row := . }}
					{{ range $cell := $row }}
						<g>
							<rect
								x="{{ mult $cell.X $cell_width }}px" 
								y="{{ mult $cell.Y $cell_height }}px" 
								width="{{ $cell_width }}px" 
								height="{{ $cell_height }}px" 
								fill="none" 
								stroke="black"
								stroke-width="1px"/>
							<text id="{{$cell.X}}-{{$cell.Y}}-value-text"
								x="{{ add (mult $cell.X $cell_width) $half_width }}px" 
								y="{{ add (mult $cell.Y $cell_height) $half_height }}px" 
								stroke="blue"
								dominant-baseline="middle" text-anchor="middle"
								>{{ printf "%.2f" $cell.Max }}</text>
						</g>
					{{ end }}
				{{ end }}
				</svg>
			</div>
		</body>
	</html>
	`); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	cells := convert_states_to_cells(server.last_update)
	if err = t.Execute(w, cells); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
}
