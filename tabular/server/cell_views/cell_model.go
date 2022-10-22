// cell_views contains views derived from the Cell view-model.
package cell_views

import (
	"math"
	"tabular/grid_world"
)

// CellViewModel is for converting the [x][y][vx][vy]State gridworld to a simpler x/y only set of cells,
// oriented in svg coordinate system such that [0][0] is the logical cell that would
// be printed in the console at top left. CellViewModel fields should be immediately usable as
// view parameters, arbitrary calculated fields can be added as desired.
type CellViewModel struct {
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
func Convert(states [][][][]grid_world.State) (cells [][]CellViewModel) {
	cells = make([][]CellViewModel, len(states))
	max_y := len(states[0])
	for x := range states {
		cells[x] = make([]CellViewModel, max_y)
	}

	maxVisitor := func(velstates [][]grid_world.State) {
		x, y := velstates[0][0].X, velstates[0][0].Y
		cellType := velstates[0][0].CellType

		maxState := grid_world.MaxVelState(velstates)
		//var maxState *grid_world.State
		// FUTURE: the below shows only the zero-velocity state for START states, which
		// are the only valid ones. This actually generalizes: only reachable states should
		// be visualized. But its a detail, I don't need the precision and just want to see overall shape.
		//if cellType == grid_world.START {
		//	// TODO: here again, finding the index of the zero-velocity is difficult because the
		//	// indices were previously aligned with the velocity values themselves.
		//	zeroVelIndex := int(math.Abs((grid_world.MAX_VELOCITY - grid_world.MIN_VELOCITY) / 2))
		//	maxState = &velstates[zeroVelIndex][zeroVelIndex]
		//} else {
		//	maxState = grid_world.MaxVelState(velstates)
		//}

		cells[x][y] = CellViewModel{
			X: x,
			// flip y indices for svg coordinate system
			Y:                   max_y - y - 1,
			Max:                 maxState.Value.AtomicRead(),
			PolicyArrowRotation: getDegrees(maxState),
			PolicyArrowScale:    getScale(maxState),
			Fill:                getFill(cellType),
		}
	}

	grid_world.VisitXYStates(states, maxVisitor)
	return
}

func getScale(state *grid_world.State) int {
	return int(math.Hypot(float64(state.VX), float64(state.VY)))
}

// getDegrees converts the vx and vy velocity components in cartesian space into the degrees passed
// to svg's rotate() transform function for an upward arrow rune. Degrees are wrt vertical.
func getDegrees(state *grid_world.State) int {
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
	case grid_world.WALL:
		fill = "lightgreen"
	case grid_world.TRACK:
		fill = "lightgray"
	case grid_world.START:
		fill = "lightblue"
	case grid_world.FINISH:
		fill = "lightyellow"
	}
	return
}
