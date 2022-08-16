// cell_views contains views which can be derived from the Cell view-model.
// Cell is merely an arbitrary representation that makes it easier to translate
// State updates to svg updates.

package cell_views

import (
	"fmt"
	"html/template"
	"strings"
	"tabular/server/fastview"

	channerics "github.com/niceyeti/channerics/channels"
)

// ValueFunction provides a view of the current value function as a 2d
// projection of the 3d function (x,y,value).
type ValueFunction struct {
	id      string
	updates <-chan []fastview.EleUpdate
}

func NewValueFunction(
	id string,
	cells <-chan [][]Cell,
	done <-chan struct{},
) (vf *ValueFunction) {
	if strings.Contains(id, "-") {
		fmt.Println("WARNING: names with hyphens intefere with html/template parsing of the `template` directive")
	}
	vf = &ValueFunction{id: template.HTMLEscapeString(id)}
	vf.updates = channerics.Convert(done, cells, vf.onUpdate)
	return
}

// TODO: Updates() is weird and seemingly trivial. Should this be done otherwise?
func (vf *ValueFunction) Updates() <-chan []fastview.EleUpdate {
	return vf.updates
}

// getPolyPoints returns an svg polygon describing these four, adjacent cells.
// Cell-A is bottom left, Cell-B is top left, Cell-C is top right, and Cell-D is bottom right.
// The polygon is projected into 2d using the lissajous transformation described in The Go Programming Language.
func getPolyPoints(
	cellA Cell,
	cellB Cell,
	cellC Cell,
	cellD Cell,
) string {
	ax, ay := cellA.X*50, cellA.Y*50
	bx, by := cellB.X*50, cellB.Y*50
	cx, cy := cellC.X*50, cellC.Y*50
	dx, dy := cellD.X*50, cellD.Y*50

	return fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d", ax, ay, bx, by, cx, cy, dx, dy)
}

func (vf *ValueFunction) Parse(
	t *template.Template,
) (name string, err error) {
	// FUTURE: disambiguate the id and template name. Conflating them like this prevents multiple instatiations of views, for instance.
	name = vf.id
	addedMap := template.FuncMap{
		"getPolyPoints": getPolyPoints,
	}
	_, err = t.Funcs(addedMap).Parse(
		`{{ define "` + name + `" }}
		<div>
			{{ $x_cells := len . }}
			{{ $y_cells := len (index . 0) }}
			{{ $num_x_polys := sub $x_cells 1 }}
			{{ $num_y_polys := sub $y_cells 1 }}
			{{ $cell_width := 100 }}
			{{ $cell_height := $cell_width }}
			{{ $width := mult $cell_width $x_cells }}
			{{ $height := mult $cell_height $y_cells }}
			{{ $half_height := div $cell_height 2 }}
			{{ $half_width := div $cell_width 2 }}
			<svg id="` + vf.id + `"
				width="{{ add $width 1 }}px"
				height="{{ add $height 1 }}px"
				style="shape-rendering: crispEdges;">
				{{ $cells := . }}
				{{ range $ri, $row := . }}
					{{ if lt $ri $num_x_polys }}
						{{ range $ci, $cell := $row }}
							{{ if lt $ci $num_y_polys }}
								<polygon 
									fill="none"
									stroke="black"
									id="{{ $cell.X }}-{{ $cell.Y }}-value-polygon"
									{{ $cell_a := index $cells $ri $ci }}
									{{ $cell_b := index $cells $ri (add $ci 1) }}
									{{ $cell_c := index $cells (add $ri 1) (add $ci 1) }}
									{{ $cell_d := index $cells (add $ri 1) $ci }}
									points="{{ getPolyPoints $cell_a $cell_b $cell_c $cell_d }}" />
							{{ end }}
						{{ end }}
					{{ end }}
				{{ end }}
			</svg>
		</div>
		{{ end }}`)
	return
}

// Returns the set of view updates needed for the view to reflect current values.
func (vf *ValueFunction) onUpdate(
	cells [][]Cell,
) (ops []fastview.EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			// Update the value text
			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-polygon", cell.X, cell.Y),
				Ops:   []fastview.Op{
					//{Key: "points", Value: fmt.Sprintf("%.2f", cell.Max)},
				},
			})
		}
	}
	return
}
