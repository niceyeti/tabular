package grid_world

import (
	"fmt"
	"math"

	"tabular/atomic_float"
)

// The state consists of the position and current x/y velocity.
// Velocity is number of cells moved per time step.
// Note that the cell type (wall, track, etc) is not really part of the state's
// identity, but is only used for the reward function.
type State struct {
	X, Y, VX, VY int
	CellType     rune
	Value        *atomic_float.AtomicFloat64
}

// Action consists of a velocity increment/decrement and horizontal or vertical direction.
// In this problem, three actions (+1, -1, 0) yields 9 actions per step, e.g. |(+1, -1, 0)|**2.
type Action struct {
	Dvx, Dvy int
}

// Step is a single SARSA time step of an agent: do action a in
// state s, observe reward r and successor s'.
type Step struct {
	// NOTE: per possible race conditions, give due consideration
	// to advantages in these being pointers or copies.
	State     *State
	Successor *State
	Action    *Action
	Reward    float64
}

// Episode is a sequence of Steps.
type Episode []Step

const (
	// Track cell types
	WALL   = 'W'
	TRACK  = 'o'
	START  = '-'
	FINISH = '+'

	// Kinematic actions in the x and y direction. A velocity of 1 means traveling one grid cell per time step.
	MAX_VELOCITY      = 4
	MIN_VELOCITY      = -MAX_VELOCITY
	NUM_VELOCITIES    = MAX_VELOCITY - MIN_VELOCITY + 1
	MAX_ACCELERATION  = 1
	MIN_ACCELERATION  = -1
	NUM_ACCELERATIONS = MAX_ACCELERATION - MIN_ACCELERATION + 1

	// Rewards
	COLLISION_REWARD = -5
	STEP_REWARD      = -1
)

// The classical track and a smaller debug track for development.
var (
	DebugTrack []string = []string{
		"WWWWWW",
		"Woooo+",
		"Woooo+",
		"WooWWW",
		"WooWWW",
		"WooWWW",
		"WooWWW",
		"W--WWW",
	}

	FullTrack []string = []string{
		"WWWWWWWWWWWWWWWWWW",
		"WWWWooooooooooooo+",
		"WWWoooooooooooooo+",
		"WWWoooooooooooooo+",
		"WWooooooooooooooo+",
		"Woooooooooooooooo+",
		"Woooooooooooooooo+",
		"WooooooooooWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WoooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWooooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWoooooooWWWWWWWW",
		"WWWWooooooWWWWWWWW",
		"WWWWooooooWWWWWWWW",
		"WWWW------WWWWWWWW",
	}
)

// Converts a track input string array to an actual state grid of positions and velocities.
// The orientation is such that the bottom/left most position of the track (when printed in a console) is (0,0).
// This gives awkward reverse-iteration displaying, but makes sense for the problem dynamics: +1 velocity yields +1 position in some array.
// Note that this is just an (X x Y x VX x VY) size matrix and would be implemented as such in Python.
// Note there is no error checking on the input track, nor error returned.
// Returns: multidim state slice, whose indices are [x][y][vx][vy].
func Convert(track []string) (states [][][][]State) {
	width := len(track[0])
	height := len(track)

	states = make([][][][]State, 0, width)
	// Build cells from left to right...
	for x := 0; x < width; x++ {
		states = append(states, make([][][]State, 0, height))
		// And bottom to top...
		for y := 0; y < height; y++ {
			states[x] = append(states[x], make([][]State, 0, NUM_VELOCITIES))
			// Select cells bottom up, so the grid has a logical progression where positive x/y velocities are right/up, from (0,0).
			cell_type := rune(track[height-y-1][x])
			// Add vx/vy velocities per x/y state
			for vxi := 0; vxi < NUM_VELOCITIES; vxi++ {
				vx := MIN_VELOCITY + vxi
				states[x][y] = append(states[x][y], make([]State, 0, NUM_VELOCITIES))
				for vy := MIN_VELOCITY; vy < NUM_VELOCITIES; vy++ {
					state := State{
						X:        x,
						Y:        y,
						VX:       vx,
						VY:       vy,
						CellType: cell_type,
						Value:    atomic_float.NewAtomicFloat64(0.0),
					}
					states[x][y][vxi] = append(states[x][y][vxi], state)
				}
			}
		}
	}

	return states
}

// A 'live' state is one for which displaying the policy is relevant information,
// e.g. is not an unreachable or invalid state.
func isLive(state *State) bool {
	return state.CellType != WALL
}

// Show the current policy, in two dimensions. Since the state space includes
// position and velocity (four dimensions), it must be projected down into two-dimensions, which makes
// sense from the perspective of driving/control. The encoding used displays a directional arrow at
// each x/y grid cell position, whose magnitude determines color of the cell. This can be done in
// html, but for displaying in a console this is truncated by simply displaying direction based on
// the maximum vx/vy value as on of ^, >, v, <.
func ShowPolicy(states [][][][]State) {
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			if isLive(&states[x][y][0][0]) {
				maxState := MaxVelState(states[x][y])
				dir := putMaxDir(maxState)
				fmt.Printf("%c %d,%d  ", dir, maxState.VX, maxState.VY)
			} else {
				fmt.Printf("-      ")
			}
		}
		fmt.Println("")
	}
}

// Show the track, for visual reference.
func ShowGrid(states [][][][]State) {
	for _, y := range Rev(len(states[0])) {
		for x := range states {
			fmt.Printf("%c ", states[x][y][0][0].CellType)
		}
		fmt.Println("")
	}
}

// Returns reversed indices of a slice, e.g. for ranging over.
func Rev(length int) []int {
	indices := make([]int, length)
	for i := 0; i < length; i++ {
		indices[i] = length - i - 1
	}
	return indices
}

// Prints the maximum vx or vy value for each x/y position in the state set.
// Note that this truncates some info, since only one of these orthogonal values sets is displayed;
// this just allows showing progress.
func ShowMaxValues(states [][][][]State) {
	fmt.Println("Max vals:")
	total := 0.0
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			state := MaxVelState(velstates)
			val := state.Value.AtomicRead()
			fmt.Printf("%.2f ", val)
			//fmt.Printf("%.2f%c ", state.value, putMaxDir(state))
			total += val
		}
		fmt.Println("")
	}
	fmt.Printf("Pi total: %.2f\n", total)
}

// Prints the average state value (over vx/vy substates) for each x/y position in the state set.
func ShowAvgValues(states [][][][]State) {
	fmt.Println("Avg vals:")
	total := 0.0
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			avg := 0.0
			n := 0.0
			for i := 0; i < len(velstates); i++ {
				for j := 0; j < len(velstates[i]); j++ {
					avg += velstates[i][j].Value.AtomicRead()
					n++
				}
			}
			avg /= n
			fmt.Printf("%.2f ", avg)
			total += avg
		}
		fmt.Println()
	}
	fmt.Printf("Total: %.2f\n", total)
}

// Returns a rune representing the max velocity direction of this state.
// This is a hyper simplified description for console-based debugging.
func putMaxDir(state *State) rune {
	// VX has greatest magnitude
	if math.Abs(float64(state.VX)) > math.Abs(float64(state.VY)) {
		if state.VX > 0 {
			return '>'
		}
		if state.VX < 0 {
			return '<'
		}
		if state.VX == 0 {
			return '='
		}
	}
	// Else, VY has the greatest magnitude
	if state.VY > 0 {
		return '^'
	}
	if state.VY < 0 {
		return 'v'
	}
	// VY == 0 or both are zero
	return '='
}

// Returns the max-valued velocity state from the subset of velocity states,
// a clumsy operation purely for viewing.
func MaxVelState(states [][]State) (maxState *State) {
	// Get the max value from the state subset of velocities
	maxState = &State{
		Value: atomic_float.NewAtomicFloat64(-math.MaxFloat64),
	}
	maxVal := maxState.Value.AtomicRead()

	for vx := range states {
		for vy := range states[vx] {
			if vx == 0 && vy == 0 {
				// Skip states whose velocity components are both zero, which are excluded by problem def,
				// except for the stationary start states.
				continue
			}

			val := states[vx][vy].Value.AtomicRead()
			if val > maxVal {
				maxState = &states[vx][vy]
				maxVal = val
			}
		}
	}

	return
}

//nolint:unused // This is sometimes useful in development.
func getStates(states [][][][]State, state_type rune) (start_states []*State) {
	accumulator := func(state *State) {
		if state.CellType == state_type {
			start_states = append(start_states, state)
		}
	}
	Visit(states, accumulator)
	return
}

// Visits every state using the passed function
func Visit(states [][][][]State, fn func(s *State)) {
	for x := range states {
		for y := range states[x] {
			for vx := range states[x][y] {
				for vy := range states[x][y][vx] {
					fn(&states[x][y][vx][vy])
				}
			}
		}
	}
}

// Visits the x/y grid states using the passed function.
func VisitXYStates(states [][][][]State, fn func(velstates [][]State)) {
	for x := range states {
		for y := range states[x] {
			fn(states[x][y])
		}
	}
}
