package models

import (
	"fmt"
	"math"

	. "tabular/atomic_helpers"
)

// The state consists of the position and current x/y velocity.
// Velocity is number of cells moved per time step.
// Note that the cell type (wall, track, etc) is not really part of the state's
// identity, but is only used for the reward function.
type State struct {
	X, Y, VX, VY int
	CellType     rune
	Value        float64
}

// Action consists of a velocity increment/decrement and horizontal or vertical direction.
// In this problem, three actions (+1, -1, 0) yields 9 actions per step, e.g. |(+1, -1, 0)|**2.
type Action struct {
	Dvx, Dvy int
}

// Step is a single SARSA time step of an agent: do action a in state s, observe reward r and successor s'.
type Step struct {
	// NOTE: per possible race conditions, give due consideration to advantages in these being pointers or copies.
	State     *State
	Successor *State
	Action    *Action
	Reward    float64
}

// Episode is a sequence of Steps.
type Episode []Step

// Track cell types
const (
	WALL   = 'W'
	TRACK  = 'o'
	START  = '-'
	FINISH = '+'
)

// Acceleration actions in the x or y direction.
const (
	MAX_VELOCITY   = 4
	MIN_VELOCITY   = 0
	NUM_VELOCITIES = 5
)

// Rewards
const (
	COLLISION_REWARD = -5
	STEP_REWARD      = -1
)

// The classical track and a smaller debug track for development.
var (
	Debug_track []string = []string{
		"WWWWWW",
		"Woooo+",
		"Woooo+",
		"WooWWW",
		"WooWWW",
		"WooWWW",
		"WooWWW",
		"W--WWW",
	}

	Full_track []string = []string{
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
	Track = Debug_track
)

// Converts a tack input string array to an actual state grid of positions and velocities.
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
			states[x] = append(states[x], make([][]State, 0, MAX_VELOCITY+1))
			// Select cells bottom up, so the grid has a logical progression where positive x/y velocities are right/up, from (0,0).
			cell_type := rune(track[height-y-1][x])
			// Augment the track cell with x/y velocity values per each state
			for vx := 0; vx < MAX_VELOCITY+1; vx++ {
				states[x][y] = append(states[x][y], make([]State, 0, MAX_VELOCITY+1)) // +1 since zero is included as a velocity.
				for vy := 0; vy < MAX_VELOCITY+1; vy++ {
					state := State{
						X:        x,
						Y:        y,
						VX:       vx,
						VY:       vy,
						CellType: cell_type,
						Value:    0,
					}
					states[x][y][vx] = append(states[x][y][vx], state)
				}
			}
		}
	}

	return states
}

// A 'live' state is one for which displaying the policy is relevant information,
// e.g. is not an unreachable or invalid state.
func is_live(state *State) bool {
	return state.CellType != WALL
}

func is_terminal(state *State) bool {
	return state.CellType == WALL || state.CellType == FINISH
}

// Show the current policy, in two dimensions. Since the state space includes
// position and velocity (four dimensions), it must be projected down into two-dimensions, which makes
// sense from the perspective of driving/control. The encoding used displays a directional arrow at
// each x/y grid cell position, whose magnitude determines color of the cell. This can be done in
// html, but for displaying in a console this is truncated by simply displaying direction based on
// the maximum vx/vy value as on of ^, >, v, <.
func Show_policy(states [][][][]State) {
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			if is_live(&states[x][y][0][0]) {
				max_state := Max_vel_state(states[x][y])
				dir := put_max_dir(max_state)
				fmt.Printf("%c %d,%d  ", dir, max_state.VX, max_state.VY)
			} else {
				fmt.Printf("-      ")
			}
		}
		fmt.Println("")
	}
}

// Show the track, for visual reference.
func Show_grid(states [][][][]State) {
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
func Show_max_values(states [][][][]State) {
	fmt.Println("Max vals:")
	total := 0.0
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			state := Max_vel_state(velstates)
			val := AtomicRead(&state.Value)
			fmt.Printf("%.2f ", val)
			//fmt.Printf("%.2f%c ", state.value, put_max_dir(state))
			total += val
		}
		fmt.Println("")
	}
	fmt.Printf("Pi total: %.2f\n", total)
}

// Prints the average state value (over vx/vy substates) for each x/y position in the state set.
func Show_avg_values(states [][][][]State) {
	fmt.Println("Avg vals:")
	total := 0.0
	for _, y := range Rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			avg := 0.0
			n := 0.0
			for i := 0; i < len(velstates); i++ {
				// From 1, since states for which both velocity components are zero or negative are excluded by problem def.
				for j := 1; j < len(velstates[i]); j++ {
					val := AtomicRead(&velstates[i][j].Value)
					avg += val
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

/*
// Purely for debugging: print the entire state structs.
func show_all(states [][][][]State, fn func(s *State) string) {
	for y := range Rev(len(states)) {
		for x := range states {
			for _, vxs := range states[x][y] {
				for _, state := range vxs {
					fmt.Printf("%s ", fn(&state))
				}
			}
			fmt.Println("")
		}
		fmt.Println("")
	}
}
*/

// Returns a printable run for the max direction value in some x/y grid position.
// This is hyper simplified for console based display.
// Note only > and ^ are possible via the problem definition, since velocity components are constrained to positive values.
func put_max_dir(state *State) rune {
	if state.VX > state.VY {
		return '>'
	}
	if state.VX < state.VY {
		return '^'
	}

	return '='
}

// Returns the max-valued velocity state from the subset of velocity states, a clumsy operation purely for viewing.
func Max_vel_state(vel_states [][]State) (max_state *State) {
	// Get the max value from the state subset of velocities
	max_state = &State{
		Value: -math.MaxFloat64,
	}
	max_val := max_state.Value

	for vx := range vel_states {
		for vy := range vel_states[vx] {
			if vx == 0 && vy == 0 {
				// Skip states whose velocity components are both zero, which are excluded by problem def.
				continue
			}

			val := AtomicRead(&vel_states[vx][vy].Value)
			if val > max_val {
				max_state = &vel_states[vx][vy]
				max_val = val
			}
		}
	}

	return
}

func get_states(states [][][][]State, state_type rune) (start_states []*State) {
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
func Visit_xy_states(states [][][][]State, fn func(velstates [][]State)) {
	for x := range states {
		for y := range states[x] {
			fn(states[x][y])
		}
	}
}
