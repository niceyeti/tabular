package cell_views

import (
	"fmt"
	"html/template"
	"strings"
	"tabular/server/fastview"

	channerics "github.com/niceyeti/channerics/channels"
)

type ValuesGrid struct {
	id      string
	updates <-chan []fastview.EleUpdate
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
	vg.updates = channerics.Convert(done, cells, vg.onUpdate)
	return
}

func (vg *ValuesGrid) Updates() <-chan []fastview.EleUpdate {
	return vg.updates
}

func (vg *ValuesGrid) Parse(
	parent *template.Template,
) (name string, err error) {
	// FUTURE: disambiguate the id and template name. Conflating them like this prevents multiple instatiations of views, for instance.
	name = vg.id
	_, err = parent.Parse(
		`{{ define "` + name + `" }}
		<div>
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
							fill="{{ $cell.Fill }}"
							stroke="black"
							stroke-width="1"/>
						<text id="{{ $cell.X }}-{{ $cell.Y }}-value-text"
							x="{{ add (mult $cell.X $cell_width) $half_width }}" 
							y="{{ add (mult $cell.Y $cell_height) (sub $half_height 10) }}" 
							stroke="blue"
							dominant-baseline="text-top" text-anchor="middle"
							>{{ printf "%.2f" $cell.Max }}</text>
						<g transform="translate({{ add (mult $cell.X $cell_width) $half_width }}, {{ add (mult $cell.Y $cell_height) (add $half_height 20)  }})">
							<text id="{{ $cell.X }}-{{ $cell.Y }}-policy-arrow"
							stroke="blue" stroke-width="1"
							dominant-baseline="central" text-anchor="middle"
							transform="rotate({{ $cell.PolicyArrowRotation }})"
							>&uarr;</text>
						</g>
					</g>
					{{ end }}
				{{ end }}
			</svg>
		</div>
		{{ end }}`)
	return
}

// Returns the set of view updates needed for the view to reflect current values.
func (vg *ValuesGrid) onUpdate(
	cells [][]Cell,
) (ops []fastview.EleUpdate) {
	for _, row := range cells {
		for _, cell := range row {
			// Update the value text
			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-value-text", cell.X, cell.Y),
				Ops: []fastview.Op{
					{
						Key:   "textContent",
						Value: fmt.Sprintf("%.2f", cell.Max),
					},
				},
			})
			// Update the policy arrow indicators
			ops = append(ops, fastview.EleUpdate{
				EleId: fmt.Sprintf("%d-%d-policy-arrow", cell.X, cell.Y),
				Ops: []fastview.Op{
					//{"transform", fmt.Sprintf("rotate(%d, %d, %d) scale(1, %d)", cell.PolicyArrowRotation, cell.X, cell.Y, cell.PolicyArrowScale)},
					{
						Key:   "transform",
						Value: fmt.Sprintf("rotate(%d)", cell.PolicyArrowRotation),
					},
					{
						Key:   "stroke-width",
						Value: fmt.Sprintf("%d", cell.PolicyArrowScale),
					},
				},
			})
		}
	}
	return
}
