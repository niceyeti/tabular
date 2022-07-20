package main

import (
	"fmt"
	"math"
)

// The state consists of the position and current x/y velocity.
// Velocity is number of cells moved per time step.
// Note that the cell type (wall, track, etc) is not really part of the state's
// identity, but is only used for the reward function.
type State struct {
	x, y, vx, vy int
	cell_type rune
	value float64
}

// The action consists of the velocity increment/decrement and horizontal or vertical direction. 
// In this problem, for three actions (+1, -1, 0), this yields 9 actions per step, e.g. |(+1, -1, 0)|**2.
type Action struct {
	dv_x, dv_y int
}

// Track cell types
const (
	WALL = 'W'
	TRACK = 'o'
	START = '-'
	FINISH = '+'
)

// Acceleration actions in the x or y direction.
const (
	X_ACC = iota
	X_DEC = iota
	X_STEADY = iota
	Y_ACC = iota
	Y_DEC = iota
	Y_STEADY = iota
)

// Rewards
const (
	COLLISION_REWARD = -5
	STEP_REWARD = -1
)

// Action directions for the policy.
const (
	UP = iota
	RIGHT = iota
	DOWN = iota
	LEFT = iota
)

// For this environment, the successor state's type (wall or track) completely determines the reward.
func reward(s_prime *State) int {
	if s_prime.cell_type == WALL {
		return COLLISION_REWARD;
	}

	return STEP_REWARD
}

// TODO: this is an input to the program. It consists only of the positional track cell info.
// It is converted to the full state set by augmenting each cell with 
var track []string = []string{
	"WWWWWW",
	"Woooo+",
	"Woooo+",
	"WooWWW",
	"WooWWW",
	"WooWWW",
	"WooWWW",
	"W--WWW",
}


/*
big_course = ['WWWWWWWWWWWWWWWWWW',
              'WWWWooooooooooooo+',
              'WWWoooooooooooooo+',
              'WWWoooooooooooooo+',
              'WWooooooooooooooo+',
              'Woooooooooooooooo+',
              'Woooooooooooooooo+',
              'WooooooooooWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WoooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWooooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWoooooooWWWWWWWW',
              'WWWWooooooWWWWWWWW',
              'WWWWooooooWWWWWWWW',
              'WWWW------WWWWWWWW']
*/

// Converts a tack input string array to an actual state grid of positions and velocities.
// The orientation is such that the bottom/left most position of the track (when printed in a console) is (0,0).
// This gives awkward reverse-iteration displaying, but makes sense for the problem dynamics: +1 velocity yields +1 position in some array.
// Note that this is just an (X x Y x VX x VY) size matrix and would be implemented as such in Python.
// Note there is no error checking on the input track, nor error returned.
// Returns: multidim state slice, whose indices are [x][y][vx][vy].
func convert_track(track []string) (states [][][][]State) {
	width := len(track[0])
	height := len(track)

	states = make([][][][]State, 0, width)
	// Build cells from left to right...
	for x := 0; x < width; x++ {
		states = append(states, make([][][]State, 0, height))
		// And bottom to top...
		for y := 0; y < height; y++ {
			states[x] = append(states[x], make([][]State, 0, 5))
			// Augment the track cell with x/y velocity values per each state
			for vx := 0; vx < 5; vx++ {
				states[x][y] = append(states[x][y], make([]State, 0, 5))
				for vy := 0; vy < 5; vy++ {
					state := State{
						x : x,
						y : y,
						vx : vx,
						vy : vy,
						cell_type: rune(track[height - y - 1][x]),
						value : 0,
					}
					states[x][y][vx] = append(states[x][y][vx], state)
				}
			}
		}
	}

	return states
}

// Show the current policy, in two dimensions. Since the state space includes 
// position and velocity (four dimensions), it must be projected down into two-dimensions, which makes
// sense from the perspective of driving/control. The encoding used displays a directional arrow at
// each x/y grid cell position, whose magnitude determines color of the cell. This can be done in 
// html, but for displaying in a console this is truncated by simply displaying direction based on
// the maximum vx/vy value as ^, >, v, <.
func show_policy(states [][][][]State) {
	for y := range rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			dir := max_dir(states[x][y])
			fmt.Printf("%c ", dir)
		}
		fmt.Println("")
	}
}

// Returns reversed indices of a slice, e.g. for ranging over.
func rev(length int) []int {
	indices := make([]int, length)
	for i := 0; i < length; i++ {
		indices[i] = length - i - 1
	}
	return indices
}

// Prints the maximum vx or vy value for each x/y position in the state set.
// Note that this truncates some info, since only one of these orthogonal values sets is displayed;
// this just allows showing progress.
func show_max_values(states [][][][]State) {
	for y := range rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			state := max_vel_state(velstates)
			fmt.Printf("%.1f ", state.value)
		}
		fmt.Println("")
	}
}

// Purely for debugging: print the entire state structs.
func show_all(states [][][][]State, fn func(s *State) string) {
	for y := range rev(len(states)) {
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

// Returns a printable run for the max direction value in some x/y grid position.
// This is hyper simplified for console based display.
// Note only > and ^ are possible via the problem definition, since velocity components are constrained to positive values.
func max_dir(vel_states[][]State) rune {
	// Get the max value from the state subset of velocities
	max_state := max_vel_state(vel_states)
	if max_state.vx > max_state.vy {
		return '>'
	}
	return '^'
}

// Returns the max-valued velocity state from the subset of velocity states, a clumsy operation.
func max_vel_state(vel_states[][]State) (max_state *State) {
	// Get the max value from the state subset of velocities
	max_state = &State {
		value: -math.MaxFloat64,
	}

	for vx := range vel_states {
		for _, state := range vel_states[vx] {
			//fmt.Printf("%.1f - \n", state.value)
			if state.value >= max_state.value {
				max_state = &state
			}
		}
	}

	return
}

// Initializes the state values
func init_state_values(states [][][][]State, value float64) {
	mutate(states, func(s *State) {s.value = value})
}

// Mutates every state using the passed function
func mutate(states [][][][]State, fn func(s *State)) {
	for _, row := range states {
		for _, col := range row {
			for _, vxs := range col {
				for i := range vxs {
					fn(&vxs[i])
				}
			}
		}
	}
}

func main() {
	// choose/input a track
	racetrack := track
	// convert to state space
	states := convert_track(racetrack)
	// initialize the state values to something slightly larger than the lowest reward, for stability
	init_state_values(states, -10)
	// display startup policy
	show_policy(states)
	// show max values
	show_max_values(states)

	//show_all(states, func(s *State) string { return fmt.Sprintf("%.1f\n", s.value)})
}