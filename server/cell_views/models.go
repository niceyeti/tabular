package cell_views

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
