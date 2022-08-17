package cell_views

import (
	"fmt"
	"html/template"
	"math"
	"strings"
	"sync"
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

var (
	// TODO: some of these are parameters that must be set per the first [][]Cell update dimensions.
	width, height float64                 // canvas size in pixels
	cellDim       float64   = 100         // Cell height/width size in pixels
	cells         float64                 // number of grid cells
	xyscale       float64                 // pixels per x or y unit
	zscale        float64                 // pixels per z unit
	ang                     = math.Pi / 8 // angle of x, y axes (e.g. =30°)
	setViewParams sync.Once = sync.Once{} // TODO: sync.Once is a code smell. This should change when views are refactored to pass in the initial [][]Cell values.
)

var sinAng, cosAng = math.Sin(ang), math.Cos(ang)

func setParams(cs [][]Cell) {
	cells = float64(len(cs))
	width = cells * cellDim
	height = float64(len(cs[0])) * cellDim
	zscale = cellDim * 0.3
	xyscale = cellDim
}

// Project applies an isometric projection to the passed points.
func project(x, y, z float64) (float64, float64) {
	sx := 1.5*width + (x-y)*cosAng*xyscale
	sy := 0.25*height + (x+y)*sinAng*xyscale - z*zscale
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
	return fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d",
		int(ax), int(ay),
		int(bx), int(by),
		int(cx), int(cy),
		int(dx), int(dy),
	)
}

// Returns the set of view updates needed for the view to reflect current values.
func (vf *ValueFunction) onUpdate(
	cells [][]Cell,
) (ops []fastview.EleUpdate) {

	setViewParams.Do(func() { setParams(cells) })

	for ri, row := range cells[:len(cells)-1] {
		for ci, cell := range row[:len(row)-1] {
			// FUTURE: (optimization) loop iteration leads to repeated calculation for many cells.
			cellA := cells[ri+1][ci]
			cellB := cells[ri][ci]
			cellC := cells[ri][ci+1]
			cellD := cells[ri+1][ci+1]

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

	return
}

// Parse returns an svg of polygons plotting that values function surface as a 2D projection.
func (vf *ValueFunction) Parse(
	t *template.Template,
) (name string, err error) {
	// FUTURE: disambiguate the id and template name. Conflating them like this prevents multiple instatiations of views, for instance.
	name = vf.id
	addedMap := template.FuncMap{
		"getPolyPoints": getPolyPoints,
	}
	// Note: the order of polygon creation forms a nice visual surface by obscuring prior polygons. Order matters.
	// Scale and height/width are also poorly parameterized, basically hardcoded to loosely center most surfaces.
	_, err = t.Funcs(addedMap).Parse(
		`{{ define "` + name + `" }}
		<div style="padding:40px;">
			{{ $x_cells := len . }}
			{{ $y_cells := len (index . 0) }}
			{{ $num_x_polys := sub $x_cells 1 }}
			{{ $num_y_polys := sub $y_cells 1 }}
			{{ $cell_width := ` + fmt.Sprintf("%d", int(cellDim)) + ` }}
			{{ $cell_height := $cell_width }}
			{{ $width := mult $cell_width $x_cells }}
			{{ $height := mult $cell_height $y_cells }}
			{{ $half_height := div $cell_height 2 }}
			{{ $half_width := div $cell_width 2 }}
			<svg id="` + vf.id + `" xmlns='http://www.w3.org/2000/svg'
				width="{{ mult $width 2 }}px"
				height="{{ mult $height 2 }}px"
				style="shape-rendering: crispEdges; stroke: black; stroke-opacity: 0.8; stroke-width: 2;">
				<g style="scale: 0.7;" >
				{{ $cells := . }}
				{{ range $ri, $row := $cells }}
					{{ if lt $ri $num_x_polys }}
						{{ range $j, $unused := $row }}
							{{ $ci := sub (sub (len $row) $j) 1 }} 
							{{ $cell := index $row $ci }}
							{{ if lt $ci $num_y_polys }}
								<polygon id="{{$cell.X}}-{{$cell.Y}}-value-polygon" fill="lightgrey" 
									{{ $cell_a := index $cells (add $ri 1) $ci }}
									{{ $cell_b := index $cells $ri $ci }}
									{{ $cell_c := index $cells $ri (add $ci 1) }}
									{{ $cell_d := index $cells (add $ri 1) (add $ci 1) }}
									points="{{ getPolyPoints $cell_a $cell_b $cell_c $cell_d }}" />
							{{ end }}
						{{ end }}
					{{ end }}
				{{ end }}
				</g>
			</svg>
		</div>
		{{ end }}`)
	return
}
