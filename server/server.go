package server

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"net/http"
	"time"

	"tabular/atomic_float"
	"tabular/models"

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

// Cell is for converting the [x][y][vx][vy]State gridworld to a simpler x/y only set of cells,
// oriented in svg coordinate system such that [0][0] is the logical cell that would
// be printed in the console at top left. This purpose of [][]Cells is convenient
// traversal and data for generating golang templates; otherwise one must implement
// ugly template funcs to map the [][][][]State structure to views, which is tedious.
// As a rule of thumb, Cell fields should be immediately usable as view parameters.
// The purpose of Cell itself is to contain ephemeral descriptors (max action direction,
// etc) useful for putting in the view, and arbitrary calculated fields can be added as desired.
type Cell struct {
	X, Y                int
	Max                 float64
	PolicyArrowRotation int
	PolicyArrowScale    int
}

// EleUpdate is an element identifier and a set of operations to apply to its attributes/content.
type EleUpdate struct {
	// The id by which to find the element
	EleId string
	// Op keys are attrib keys or 'textContent', values are the strings to which these are set.
	// Example: ('x','123') means 'set attribute 'x' to 123. 'textContent' is a reserved key:
	// ('textContent','abc') means 'set ele.textContent to abc'.
	Ops []Op
}

// Op is a key and value. For example an html attribute and its new value.
type Op struct {
	Key   string
	Value string
}

type Server struct {
	addr          string
	last_update   [][][][]models.State
	state_updates <-chan [][][][]models.State
	views         []ViewComponent
}

/*
Server: this server is a monolith. A pure server would abstract away the details of each
visual component from some builder/factories for generating them (and their websockets),
and would then simply coordinate them. This server instead has it all: knowledge of
templates, converting models to view models, and bootstrapping web sockets.
*/

func NewServer(
	addr string,
	initial_states [][][][]models.State,
	state_updates <-chan [][][][]models.State) *Server {
	return &Server{
		addr:          addr,
		last_update:   initial_states,
		state_updates: state_updates,
		views: []ViewComponent{
			NewValuesGrid("valuesgrid"),
		},
	}
}

type ViewComponent interface {
	// TODO: no error needed, could use Must() instead of returning one... or maybe caller would do so
	Template(template.FuncMap) (*template.Template, error)
	Update([][]Cell) []EleUpdate
}

type ValuesGrid struct {
	name string
}

func NewValuesGrid(name string) *ValuesGrid {
	name = template.HTMLEscapeString(name)
	return &ValuesGrid{name}
}

func (vg *ValuesGrid) Template(func_map template.FuncMap) (t *template.Template, err error) {
	return template.New(vg.name).Funcs(func_map).Parse(
		`<div id="state_values">
			{{ $x_cells := len . }}
			{{ $y_cells := len (index . 0) }}
			{{ $cell_width := 100 }}
			{{ $cell_height := $cell_width }}
			{{ $width := mult $cell_width $x_cells }}
			{{ $height := mult $cell_height $y_cells }}
			{{ $half_height := div $cell_height 2 }}
			{{ $half_width := div $cell_width 2 }}
			<svg id="` + vg.name + `"
				width="{{ add $width 1 }}px"
				height="{{ add $height 1 }}px"
				style="shape-rendering: crispEdges;">
				{{ range $row := . }}
					{{ range $cell := $row }}
					<g>
						<rect
							x="{{ mult $cell.X $cell_width }}" 
							y="{{ mult $cell.Y $cell_height }}"
							width="{{ $cell_width }}"
							height="{{ $cell_height }}" 
							fill="none"
							stroke="black"
							stroke-width="1"/>
						<text id="{{$cell.X}}-{{$cell.Y}}-value-text"
							x="{{ add (mult $cell.X $cell_width) $half_width }}" 
							y="{{ add (mult $cell.Y $cell_height) (sub $half_height 10) }}" 
							stroke="blue"
							dominant-baseline="text-top" text-anchor="middle"
							>{{ printf "%.2f" $cell.Max }}</text>
						<g transform="translate({{ add (mult $cell.X $cell_width) $half_width }}, {{ add (mult $cell.Y $cell_height) (add $half_height 20)  }})">
							<text id="{{$cell.X}}-{{$cell.Y}}-policy-arrow"
							stroke="blue" stroke-width="1"
							dominant-baseline="central" text-anchor="middle"
							transform="rotate({{ $cell.PolicyArrowRotation }})"
							>&uarr;</text>
						</g>
					</g>
					{{ end }}
				{{ end }}
			</svg>
		</div>`)
}

// Returns the set of view updates needed for the view to reflect the current values.
func (vg *ValuesGrid) Update(cells [][]Cell) (ops []EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			// Update the value text
			ops = append(ops, EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-text", cell.X, cell.Y),
				Ops: []Op{
					{"textContent", fmt.Sprintf("%.2f", cell.Max)},
				},
			})
			// Update the policy arrow indicators
			ops = append(ops, EleUpdate{
				EleId: fmt.Sprintf("%d-%d-policy-arrow", cell.X, cell.Y),
				Ops: []Op{
					//{"transform", fmt.Sprintf("rotate(%d, %d, %d) scale(1, %d)", cell.PolicyArrowRotation, cell.X, cell.Y, cell.PolicyArrowScale)},
					{"transform", fmt.Sprintf("rotate(%d)", cell.PolicyArrowRotation)},
					{"stroke-width", fmt.Sprintf("%d", cell.PolicyArrowScale)},
				},
			})
		}
	}
	return
}

func convert_states_to_cells(states [][][][]models.State) (cells [][]Cell) {
	cells = make([][]Cell, len(states))
	max_y := len(states[0])
	for x := range states {
		cells[x] = make([]Cell, max_y)
	}

	models.Visit_xy_states(states, func(velstates [][]models.State) {
		x, y := velstates[0][0].X, velstates[0][0].Y
		maxState := models.Max_vel_state(velstates)
		// flip the y indices for displaying in svg coordinate system
		cells[x][y] = Cell{
			X: x, Y: max_y - y - 1,
			Max:                 atomic_float.AtomicRead(&maxState.Value),
			PolicyArrowRotation: getDegrees(maxState),
			PolicyArrowScale:    getScale(maxState),
		}
	})
	return
}

func getScale(state *models.State) int {
	return int(math.Hypot(float64(state.VX), float64(state.VY)))
}

// getDegrees converts the vx and vy velocity components in cartesian space into the degrees passed
// to svg's rotate() transform function for an upward arrow rune. Degrees are wrt vertical.
func getDegrees(state *models.State) int {
	if state.VX == 0 && state.VY == 0 {
		return 0
	}
	rad := math.Atan2(float64(state.VY), float64(state.VX))
	deg := rad * 180 / math.Pi
	// deg is correct in cartesian space, but must be subtracted from 90 for rotation in svg coors
	return int(90 - deg)
}

// Returns the set of view updates needed for the view to reflect the current values.
func get_cell_updates(cells [][]Cell) (updates []EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			// Update the value text
			updates = append(updates, EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-text", cell.X, cell.Y),
				Ops: []Op{
					{"textContent", fmt.Sprintf("%.2f", cell.Max)},
				},
			})
			// Update the policy arrow indicators
			updates = append(updates, EleUpdate{
				EleId: fmt.Sprintf("%d-%d-policy-arrow", cell.X, cell.Y),
				Ops: []Op{
					//{"transform", fmt.Sprintf("rotate(%d, %d, %d) scale(1, %d)", cell.PolicyArrowRotation, cell.X, cell.Y, cell.PolicyArrowScale)},
					{"transform", fmt.Sprintf("rotate(%d)", cell.PolicyArrowRotation)},
					{"stroke-width", fmt.Sprintf("%d", cell.PolicyArrowScale)},
				},
			})
		}
	}
	return
}

func (server *Server) Serve() {
	http.HandleFunc("/", server.serve_index)
	http.HandleFunc("/ws", server.serve_websocket)
	//http.HandleFunc("/profile", pprof.Profile)
	// TODO: parameterize port, addr per container requirements. The client bootstrap code must also receive
	// the port number to connect to the web socket.
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println(err)
	}
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

// publish_state_updates transforms state updates from the training method into
// view updates sent to the client.
// Note that taking too long here could block senders on the
// state chan; this will surely change as code develops, be mindful of upstream effects.
func (server *Server) publish_state_updates(ws *websocket.Conn) {
	publish := func(updates []EleUpdate) <-chan error {
		errchan := make(chan error)
		go func() {
			defer close(errchan)
			if err := ws.WriteJSON(updates); err != nil {
				errchan <- err
			}
		}()
		return errchan
	}
	last_update_time := time.Now()
	resolution := time.Millisecond * 200
	var done <-chan error

	// Watch for state updates and push them to the client.
	// Updates are published per a max number of updates per second.
	for states := range server.state_updates {
		//fmt.Println("WS server tick")
		// Drop updates when receiving them too quickly.
		if time.Since(last_update_time) < resolution {
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

		last_update_time = time.Now()
		if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			fmt.Println(err)
			return
		}

		cells := convert_states_to_cells(states)
		updates := get_cell_updates(cells)
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
/*
Each component:
	- has-a static template in which it embeds its initial parameters
	- receives latest states and generates update ops

	Methods:
	Template([][]Cell) (*t.Template, error) // The initial template of the component, given the cells
	Update(states [][]Cell) ([]Op)  // Notify of the latest cell values and get updates




*/
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
	// is weird about passing this down. A shared func map smells like distorted encapsulation.
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

	cells := convert_states_to_cells(server.last_update)
	if err = t.Execute(w, cells); err != nil {
		_, _ = w.Write([]byte(err.Error()))
		return
	}
}
