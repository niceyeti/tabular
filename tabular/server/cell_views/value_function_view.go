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
	done <-chan struct{},
	cells <-chan [][]Cell,
) (vf *ValueFunction) {
	id := "valuefunction"
	if strings.Contains(id, "-") {
		fmt.Println("WARNING: hyphenated interfere with html/template's `template` directive")
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
	width, height float64      // canvas size in pixels
	cellDim       float64 = 80 // Cell height/width size in pixels
	cells         float64      // number of grid cells
	xyscale       float64      // pixels per x or y unit
	zscale        float64      // pixels per z unit
	// ang could easily be a dynamic parameter, from the user or otherwise, for a fixed set of view angles (30, 45, etc.)
	ang                     = math.Pi / 6 // angle of x, y axes (e.g. =30°)
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
	sx := (x - y) * cosAng * xyscale
	sy := (x+y)*sinAng*xyscale - z*zscale
	return sx, sy
}

// Cell-A is bottom left, Cell-B is top left, Cell-C is top right, and Cell-D is bottom right.
// The polygon is projected into 2d using the lissajous transformation described in The Go Programming Language.
func getPolyPoints(
	cellA Cell,
	cellB Cell,
	cellC Cell,
	cellD Cell,
) string {
	return makeFuncPolygon("", cellA, cellB, cellC, cellD).String()
}

// Returns an svg polygon describing these four, adjacent cells.
// The polygon is projected into 2d using a similar to the lissajous transformation described in The Go Programming Language.
func makeFuncPolygon(
	id string,
	cellA Cell,
	cellB Cell,
	cellC Cell,
	cellD Cell,
) (fp *funcPolygon) {
	fp = &funcPolygon{
		Id: id,
	}
	fp.ax, fp.ay = project(float64(cellA.X), float64(cellA.Y), cellA.Max)
	fp.bx, fp.by = project(float64(cellB.X), float64(cellB.Y), cellB.Max)
	fp.cx, fp.cy = project(float64(cellC.X), float64(cellC.Y), cellC.Max)
	fp.dx, fp.dy = project(float64(cellD.X), float64(cellD.Y), cellD.Max)
	return
}

type funcPolygon struct {
	Id     string
	ax, ay float64
	bx, by float64
	cx, cy float64
	dx, dy float64
}

// String returns a string suitable for the svg-polygon 'points' attribute.
// The values are truncated to ints, which is a bit of premature svg-optimization.
func (fp *funcPolygon) String() string {
	return fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d",
		int(fp.ax), int(fp.ay),
		int(fp.bx), int(fp.by),
		int(fp.cx), int(fp.cy),
		int(fp.dx), int(fp.dy),
	)
}

func minFour(f1, f2, f3, f4 float64) float64 {
	return math.Min(
		math.Min(f1, f2),
		math.Min(f3, f4),
	)
}

func maxFour(f1, f2, f3, f4 float64) float64 {
	return math.Max(
		math.Max(f1, f2),
		math.Max(f3, f4),
	)
}

func (fp *funcPolygon) MinX() float64 {
	return minFour(fp.ax, fp.bx, fp.cx, fp.dx)
}

func (fp *funcPolygon) MinY() float64 {
	return minFour(fp.ay, fp.by, fp.cy, fp.dy)
}

func (fp *funcPolygon) MaxX() float64 {
	return maxFour(fp.ax, fp.bx, fp.cx, fp.dx)
}

func (fp *funcPolygon) MaxY() float64 {
	return maxFour(fp.ay, fp.by, fp.cy, fp.dy)
}

func avg(f ...float64) float64 {
	n, sum := 0.0, 0.0
	for _, fn := range f {
		sum += fn
		n++
	}
	return sum / n
}

// Returns the set of view updates needed for the view to reflect current values.
func (vf *ValueFunction) onUpdate(
	cells [][]Cell,
) (ops []fastview.EleUpdate) {
	// TODO: refactor to move/remove
	setViewParams.Do(func() { setParams(cells) })

	// Get the min and max function values, for plotting pseudo-gradients on the surface.
	// These determine the logical stop points of the gradient extremes; each polygon is
	// manually shaded with the average of its four max-values. The alternative to this is
	// that each polygon has-a linear-gradient than it updates, using some complex math.
	minVal, maxVal := math.MaxFloat64, -math.MaxFloat64
	for _, row := range cells {
		for _, cell := range row {
			minVal = math.Min(minVal, cell.Max)
			maxVal = math.Max(maxVal, cell.Max)
		}
	}

	// First build up the polygons, so we can later center their svg coordinates within the view axe.
	xmin, ymin := math.MaxFloat64, math.MaxFloat64
	xmax, ymax := -math.MaxFloat64, -math.MaxFloat64
	for ri, row := range cells[:len(cells)-1] {
		for ci, cell := range row[:len(row)-1] {
			// FUTURE: (optimization) loop iteration leads to repeated calculation for many cells.
			cellA := cells[ri+1][ci]
			cellB := cells[ri][ci]
			cellC := cells[ri][ci+1]
			cellD := cells[ri+1][ci+1]
			polygon := makeFuncPolygon(
				fmt.Sprintf("%d-%d-value-polygon", cell.X, cell.Y),
				cellA, cellB, cellC, cellD,
			)

			xmin = math.Min(xmin, polygon.MinX())
			xmax = math.Max(xmax, polygon.MaxX())

			ymin = math.Min(ymin, polygon.MinY())
			ymax = math.Max(ymax, polygon.MaxY())

			avgVal := avg(cellA.Max, cellB.Max, cellC.Max, cellD.Max)
			fill := getRGBFill(avgVal, minVal, maxVal)

			ops = append(ops, fastview.EleUpdate{
				EleId: polygon.Id,
				Ops: []fastview.Op{
					{
						Key:   "points",
						Value: polygon.String(),
					},
					{
						Key:   "fill",
						Value: fill,
					},
				},
			})
		}
	}

	// Shift all values by the min x and y to center the view, and scale it down to fit.
	// FWIW, this could be done using fewer computations with an enclosing <g transform="translate(minx, miny)">

	// Scale down by the maximum required to fit the full plot in view, but only if needed (when scaler < 1.0)
	scaler := math.Min(
		math.Min(
			math.Abs(width/(xmax-xmin)),
			math.Abs(height/(ymax-ymin)),
		),
		1.0,
	)

	ops = append(ops, fastview.EleUpdate{
		EleId: vf.id + "-group",
		Ops: []fastview.Op{
			{
				Key:   "transform",
				Value: fmt.Sprintf("scale(%f) translate(%d %d)", scaler, int(-xmin), int(-ymin)),
			},
		},
	})

	return
}

// Returns an RGB value defined by where avgVal lies along the number line between minVal and maxVal.
// Some proportion of RGB values is assigned based on this relative position.
func getRGBFill(avgVal, minVal, maxVal float64) string {
	// Allocate fill based on proportion of blue and red only; this should give a basic relative range.
	redPct := int(100.0 * math.Abs(avgVal) / math.Abs(maxVal-minVal))
	return fmt.Sprintf("rgb(%d%%,0%%,%d%%)", redPct, 100-redPct)
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
				style="shape-rendering: crispEdges; stroke: lightgrey; stroke-opacity: 1.0; stroke-width: 3;">
				<g id="` + vf.id + "-group" + `" transform="translate(0 0)">
				{{ $cells := . }}
				{{ range $ri, $row := $cells }}
					{{ if lt $ri $num_x_polys }}
						{{ range $j, $unused := $row }}
							{{ $ci := sub (sub (len $row) $j) 1 }} 
							{{ $cell := index $row $ci }}
							{{ if lt $ci $num_y_polys }}
								<polygon id="{{$cell.X}}-{{$cell.Y}}-value-polygon"
									fill="black" fill-opacity="1.0"
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
