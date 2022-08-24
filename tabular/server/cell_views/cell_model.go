// cell_views contains views derived from the Cell view-model.
package cell_views

import (
	"math"
	"tabular/models"
)

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
	Fill                string
}

// Convert transforms the passed state models into Cells for consumption by values-views.
// The y indices into [][]Cell matrix are flipped per svg y-axis orientation, where 0 is the top of
// the coordinate system.
// TODO: where can this live? Is reorg needed? Notice how this references model.State and helpers.
// I suppose this is fine, but re-evaluate.
func Convert(states [][][][]models.State) (cells [][]Cell) {
	cells = make([][]Cell, len(states))
	max_y := len(states[0])
	for x := range states {
		cells[x] = make([]Cell, max_y)
	}

	models.VisitXYStates(states, func(velstates [][]models.State) {
		x, y := velstates[0][0].X, velstates[0][0].Y
		cellType := velstates[0][0].CellType
		maxState := models.MaxVelState(velstates)
		// flip the y indices for displaying in svg coordinate system
		cells[x][y] = Cell{
			X:                   x,
			Y:                   max_y - y - 1,
			Max:                 maxState.Value.AtomicRead(),
			PolicyArrowRotation: getDegrees(maxState),
			PolicyArrowScale:    getScale(maxState),
			Fill:                getFill(cellType),
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

func getFill(cellType rune) (fill string) {
	switch cellType {
	case models.WALL:
		fill = "lightgreen"
	case models.TRACK:
		fill = "lightgray"
	case models.START:
		fill = "lightblue"
	case models.FINISH:
		fill = "lightyellow"
	}
	return
}
