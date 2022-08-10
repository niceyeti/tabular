// cell_views contains views which can be derived from the Cell view-model.
// Cell is merely an arbitrary representation that makes it easier to translate
// State updates to svg updates.

package cell_views

import (
	"fmt"
	"html/template"
	"tabular/server/fastview"
)

var _ *fastview.View = nil

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

type ViewComponent interface {
	// TODO: no error needed, could use Must() instead of returning one... or maybe caller would do so
	Template(template.FuncMap) (*template.Template, error)
	Update([][]Cell) []fastview.EleUpdate
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
func (vg *ValuesGrid) Update(cells [][]Cell) (ops []fastview.EleUpdate) {
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
