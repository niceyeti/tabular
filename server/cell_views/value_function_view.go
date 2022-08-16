// cell_views contains views which can be derived from the Cell view-model.
// Cell is merely an arbitrary representation that makes it easier to translate
// State updates to svg updates.

package cell_views

import (
	"fmt"
	"html/template"
	"math"
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

const (
	// TODO: some of these are parameters that must be set per the first [][]Cell update dimensions.
	width, height = 600, 320            // canvas size in pixels
	cells         = 10                  // number of grid cells
	xyrange       = 10.0                // axis ranges (-xyrange..+xyrange)
	xyscale       = width / 2 / xyrange // pixels per x or y unit
	zscale        = height * 0.4        // pixels per z unit
	angle         = math.Pi / 6         // angle of x, y axes (=30°)
)

var sin30, cos30 = math.Sin(angle), math.Cos(angle) // sin(30°), cos(30°)

// Project applies an isometric projection to the passed points.
func project(x, y, z float64) (float64, float64) {
	// Scale x and y.
	// TODO: scaling probably belong outide of project().
	x = xyrange * (x/cells - 0.5)
	y = xyrange * (y/cells - 0.5)

	sx := width/2.0 + (x-y)*cos30*xyscale
	sy := height/2.0 + (x+y)*sin30 - x*zscale

	return sx, sy
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
	ax, ay := project(float64(cellA.X), float64(cellA.Y), cellA.Max)
	bx, by := project(float64(cellB.X), float64(cellB.Y), cellB.Max)
	cx, cy := project(float64(cellC.X), float64(cellC.Y), cellC.Max)
	dx, dy := project(float64(cellD.X), float64(cellD.Y), cellD.Max)

	// TODO: redo with vals truncated to ints. Or floats... int may be premature optimization.
	return fmt.Sprintf("%f,%f %f,%f %f,%f %f,%f", ax, ay, bx, by, cx, cy, dx, dy)
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
									id="{{$cell.X}}-{{$cell.Y}}-value-polygon"
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
	for ri, row := range cells[:len(cells)-1] {
		for ci, cell := range row[:len(row)-1] {
			cellA := cells[ri][ci]
			cellB := cells[ri][ci+1]
			cellC := cells[ri+1][ci+1]
			cellD := cells[ri+1][ci]

			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-polygon", cell.X, cell.Y),
				Ops: []fastview.Op{
					{
						Key:   "points",
						Value: getPolyPoints(cellA, cellB, cellC, cellD),
					},
				},
			})
		}
	}

	//fmt.Println(ops)
	return
}
