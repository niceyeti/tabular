// cell_views contains views which can be derived from the Cell view-model.
// Cell is merely an arbitrary representation that makes it easier to translate
// State updates to svg updates.

package cell_views

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"tabular/atomic_float"
	"tabular/models"
	"tabular/server/fastview"

	channerics "github.com/niceyeti/channerics/channels"
)

// TODO: remove
var _ *fastview.Op = nil

// Convert transforms the passed state models into Cells for consumption by values-views.
// TODO: where can this live? Is reorg needed? Notice how this references model.State and helpers.
// I suppose this is fine, but re-evaluate.
func Convert(states [][][][]models.State) (cells [][]Cell) {
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

type ValuesGrid struct {
	id      string
	updates chan []fastview.EleUpdate
}

func NewValuesGrid(
	id string,
	cells <-chan [][]Cell,
	done <-chan struct{},
) (vg *ValuesGrid) {
	if strings.Contains(id, "-") {
		fmt.Println("WARNING: names with hyphens intefere with html/template parsing of the `template` directive")
	}
	vg = &ValuesGrid{id: template.HTMLEscapeString(id)}
	// Init is a bit awkward, this could also be done via lazy-init in the Updates() method.
	vg.init(done, cells)

	return
}

func (vg *ValuesGrid) init(
	done <-chan struct{},
	cells <-chan [][]Cell,
) {
	if vg.updates != nil {
		return
	}

	updates := make(chan []fastview.EleUpdate)
	go func() {
		defer close(updates)
		for nextCells := range channerics.OrDone(done, cells) {
			eleOps := vg.update(nextCells)
			select {
			case updates <- eleOps:
			case <-done:
				return
			}
		}
	}()
	vg.updates = updates
}

func (vg *ValuesGrid) Updates() <-chan []fastview.EleUpdate {
	return vg.updates
}

func (vg *ValuesGrid) Template(
	func_map template.FuncMap,
) (t *template.Template, err error) {
	return template.New(vg.id).Funcs(func_map).Parse(
		`<div id="state_values">
			{{ $x_cells := len . }}
			{{ $y_cells := len (index . 0) }}
			{{ $cell_width := 100 }}
			{{ $cell_height := $cell_width }}
			{{ $width := mult $cell_width $x_cells }}
			{{ $height := mult $cell_height $y_cells }}
			{{ $half_height := div $cell_height 2 }}
			{{ $half_width := div $cell_width 2 }}
			<svg id="` + vg.id + `"
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
func (vg *ValuesGrid) update(cells [][]Cell) (ops []fastview.EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			// Update the value text
			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-text", cell.X, cell.Y),
				Ops: []fastview.Op{
					{Key: "textContent", Value: fmt.Sprintf("%.2f", cell.Max)},
				},
			})
			// Update the policy arrow indicators
			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-policy-arrow", cell.X, cell.Y),
				Ops: []fastview.Op{
					//{"transform", fmt.Sprintf("rotate(%d, %d, %d) scale(1, %d)", cell.PolicyArrowRotation, cell.X, cell.Y, cell.PolicyArrowScale)},
					{Key: "transform", Value: fmt.Sprintf("rotate(%d)", cell.PolicyArrowRotation)},
					{Key: "stroke-width", Value: fmt.Sprintf("%d", cell.PolicyArrowScale)},
				},
			})
		}
	}
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
