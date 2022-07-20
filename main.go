package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// The state consists of the position and current x/y velocity.
// Velocity is number of cells moved per time step.
// Note that the cell type (wall, track, etc) is not really part of the state's
// identity, but is only used for the reward function.
type State struct {
	x, y, vx, vy int
	cell_type    rune
	value        float64
}

// The action consists of the velocity increment/decrement and horizontal or vertical direction.
// In this problem, for three actions (+1, -1, 0), this yields 9 actions per step, e.g. |(+1, -1, 0)|**2.
type Action struct {
	dvx, dvy int
}

// A step is a single SARSA time step of an agent: do action a in state s, observe reward r and successor state s'.
type Step struct {
	// NOTE: for the sake of parallel training, give due consideration to advantages in these being pointers or copies.
	state     *State
	successor *State
	action    *Action
	reward    float64
}

// An episode is merely a sequence of Steps.
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
	MAX_VELOCITY = 4
	MIN_VELOCITY = 0
)

// Rewards
const (
	COLLISION_REWARD = -5
	STEP_REWARD      = -1
)

/*
// Action directions for the policy.
const (
	UP    = iota
	RIGHT = iota
	DOWN  = iota
	LEFT  = iota
)
*/

// TODO: this is an input to the program. It consists only of the positional track cell info.
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
			states[x] = append(states[x], make([][]State, 0, MAX_VELOCITY+1))
			// Select cells bottom up, so the grid has a logical progression where positive x/y velocities are right/up, from (0,0).
			cell_type := rune(track[height-y-1][x])
			// Augment the track cell with x/y velocity values per each state
			for vx := 0; vx < MAX_VELOCITY+1; vx++ {
				states[x][y] = append(states[x][y], make([]State, 0, MAX_VELOCITY+1)) // +1 since zero is included as a velocity.
				for vy := 0; vy < MAX_VELOCITY+1; vy++ {
					state := State{
						x:         x,
						y:         y,
						vx:        vx,
						vy:        vy,
						cell_type: cell_type,
						value:     0,
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
	for _, y := range rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			dir := max_dir(states[x][y])
			fmt.Printf("%c ", dir)
		}
		fmt.Println("")
	}
}

// Show the track, for visual reference.
func show_grid(states [][][][]State) {
	for _, y := range rev(len(states[0])) {
		for x := range states {
			fmt.Printf("%c ", states[x][y][0][0].cell_type)
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
	for _, y := range rev(len(states[0])) {
		fmt.Print(" ")
		for x := range states {
			velstates := states[x][y]
			state := max_vel_state(velstates)
			fmt.Printf("%.1f ", state.value)
		}
		fmt.Println("")
	}
}

/*
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
*/

// Returns a printable run for the max direction value in some x/y grid position.
// This is hyper simplified for console based display.
// Note only > and ^ are possible via the problem definition, since velocity components are constrained to positive values.
func max_dir(vel_states [][]State) rune {
	// Get the max value from the state subset of velocities
	max_state := max_vel_state(vel_states)
	if max_state.vx > max_state.vy {
		return '>'
	}
	return '^'
}

// Returns the max-valued velocity state from the subset of velocity states, a clumsy operation.
func max_vel_state(vel_states [][]State) (max_state *State) {
	// Get the max value from the state subset of velocities
	max_state = &State{
		value: -math.MaxFloat64,
	}

	for vx := range vel_states {
		for vy := range vel_states[vx] {
			state := vel_states[vx][vy]
			//fmt.Printf("%.1f - \n", state.value)
			if state.vx > 0 && state.vy > 0 { // Skip states whose velocity components are both zero, excluded by problem def.
				if state.value >= max_state.value {
					max_state = &state
				}
			}
		}
	}

	return
}

// Initializes the state values
func init_state_values(states [][][][]State, value float64) {
	mutate(states, func(s *State) { s.value = value })
}

// Mutates every state using the passed function
func mutate(states [][][][]State, fn func(s *State)) {
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

/*
- Each agent generates episodes and then updates state values based on them
- To avoid locking the state values I need to refactor
  * the critical region is the state value; lack of coordination will cause lost/incorrect updates
    * copy the values when the agent visits them? this has the same lost-update problem, even if threadsafe
  * the work being performed is the generation of episodes; these could be sent back to a processing worker(s) which performs the update
    * paralellizing the updates could be performed by:
      * separating sequences by disjoint state sets
      * only update states the first time they are encountered in the incoming data (mark and update; agent's could unmark states as candidates, though this is another race condition...)
      * discard updates to states that have been updated since time
- Batch and process: run almost like a garbage collector, where every so often the processor tells the agents to pause while it sweeps
and processes episodes, updating the state values.
- Give each agent a different start state: this reduces interference by ensuring their trajectories are not totally overlapping, but does not eliminate it.
- Scioning: agents abort episodes (maintaining state up to that point) when they reach a state another agent has visited.
  Since their policies are the same/similar, merging these episodes is mathematically sound.
- Agents go off-policy: agents do not transition to states that have already been visited. An off-policy scheme means they could continue to learn and gather info.
- Agents update values themselves (as copies), then queue these to the processor which could act as an update broker in some manner.
- Vanilla, uncoordinated version: agents generate episode sequences and send these to the processor which merely processes them one at a time.
  Any discrepancies in learning/estimation might be expected to be dominated by progress in the algorithm. The processor periodically
  halts the agents to update the values. This version assumes a large state space with minimal interference.

Lessons: the critical region of parallel MC is the state values. Agents use them to act, even if training off-policy, thus
updating them has two issues: 1) they might be in use, and updating invalidates an episode 2) updating them is unstable if
episodes contain state overlap.
- One possible solution is to batch training episodes together, then average their combined updates to overlapping state values, per processing step.
  This algorithm looks like this:
    - run: generate a bunch of episodes using the current state values. The generation method (policy) should maximize information gathering:
      each agent implements a multistep policy of some kind, like striping in deep learning. Generation could halt at some threshold, such as
      repeated state visits, goals reached, or merely time.
    - learn: pause the agents at the start line, and perform all of these updates, averaging the value updates together.

A problem for parallization is that the non-mutual updates to states mean agents compete to perform updates; thus agents are interfering with
eachother's policies, e.g. policies are no longer independent. This would likely become most problematic near convergence.


Note: the vanilla batching approach to parallelism satisfies the requirement of coordination between the agents and the
processor updating the state values: the processor pauses the agents and then runs to completion. Again this is also mathematically
sound since the values need to be quiescent for the agents to act and generate valuable information. A more efficient
and less chunky coordination mechanism may be possible (as always, don't overthink) but I like how this approach is amenable
to other bootstrap methods like Q-learning. Note that its resemblance to the policy-evaluation/policy-improvement DP algorithm
suggests there may be a meta intepretation, e.g., the agents are doing evaluation, the processor is doing improvement, and perhaps
somehow thereby could be refactored to a more efficient coordination scheme. Also, there may be a mathematical way to resolve
conflicting updates in a manner such that no coordination is needed (?). If agent behavior is purely off-policy (e.g. Q-learning)
then coordination doesn't matter and actions determined by obsolete values don't matter; but at that point I'll just implement Q.
The intent of a-MC is purely to play with MC and search/initialization methods for optimizing convergence, just note how this
convo directly leads to Q-learning and other methods.

Note: For MC consider implementing an 'oracle' agent whose policy satsifies some basic conditions. The best oracle would be an
example trajectory on the shortest path from start to finish; others might be an obstacle aware oracle or a min-cost oracle
with map awareness. This heuristic is to enable metalearning from human examples, to avoid the problem of purely 'dumb' initial
agent's that will take an eternity to randomly reach some goal state and thereby propagate useful information back.



*/

func visit(states [][][][]State, fn func(*State)) {
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

func get_start_states(states [][][][]State) (start_states []*State) {
	accumulator := func(state *State) {
		if state.cell_type == START {
			start_states = append(start_states, state)
		}
	}
	visit(states, accumulator)
	return
}

// Gets the successor state given the domain kinematics: current position plus
// x/y velocity, plus collision constraints, equals new state.
func get_successor(
	states [][][][]State,
	cur_state *State,
	action *Action,
) (successor *State) {
	// For bounds checking, to confine the agent within the grid.
	max_x := float64(len(states) - 1)
	max_y := float64(len(states[0]) - 1)
	// Get the proposed velocity per this Action, min of 0 and max of 4 per problem definition.
	// Though it is a little odd that the state-encoding does not encompass the action, this is
	// normal for MC, for which only state value estimates are of concern, not Q(s,a) values.
	// Logically, however, the consequence of the action *is* stored in the next state's encoding.
	new_vx := int(math.Max(math.Min(float64(cur_state.vx+action.dvx), MAX_VELOCITY), MIN_VELOCITY))
	new_vy := int(math.Max(math.Min(float64(cur_state.vy+action.dvy), MAX_VELOCITY), MIN_VELOCITY))
	// Get new x/y position, bounded by the grid.
	new_x := int(math.Max(math.Min(float64(cur_state.x+new_vx), max_x), 0))
	new_y := int(math.Max(math.Min(float64(cur_state.y+new_vy), max_y), 0))

	successor = &states[new_x][new_y][new_vx][new_vy]
	//fmt.Println("States:\n", cur_state, "\n", successor, "\n", action)
	return
}

func get_rand_dv() int {
	return rand.Int()%3 - 1
}

func get_rand_action(cur_state *State) (action *Action) {
	action = &Action{
		dvx: get_rand_dv(),
		dvy: get_rand_dv(),
	}
	// By problem def, velocity components cannot both be zero, so the effect of this action must be checked.
	for cur_state.vx+action.dvx == 0 && cur_state.vy+action.dvy == 0 {
		action.dvx = get_rand_dv()
		action.dvy = get_rand_dv()
	}
	return
}

func get_reward(target *State) (reward float64) {
	switch target.cell_type {
	case WALL:
		reward = COLLISION_REWARD
	case TRACK, FINISH:
		reward = STEP_REWARD
	case START:
		reward = STEP_REWARD
	default:
		// Degenerate case; this is unreachable code if all actions are covered in switch.
		panic("Shazbot!")
	}
	return
}

func is_terminal(state *State) bool {
	return state.cell_type == WALL || state.cell_type == FINISH
}

/*
Implements vanilla alpha-MC using a fixed number of workers to generate episodes
which are sent to the estimator to update the state values. Coordination is simple:
	- agents generate and queue episodes up to some stopping criteria
	- processor halts the agents to empty its episode queue and update state values
*/
func alpha_mc_train_vanilla_parallel(states [][][][]State, nworkers int) {
	start_states := get_start_states(states)
	rand_restart := func() *State {
		ri := rand.Int() % len(start_states)
		return start_states[ri]
	}

	alpha := 0.1
	gamma := 0.9
	policy_alpha_max := func(state *State) (target *State, action *Action) {
		r := rand.Float64()
		if r <= alpha {
			// do something random
			action := get_rand_action(state)
			target = get_successor(states, state, action)
		} else {
			// Search for max-valued state per available actions.
			max_val := -math.MaxFloat64
			for dvx := -1; dvx < 2; dvx++ {
				for dvy := -1; dvy < 2; dvy++ {
					// Get the successor state and its value; trad MC does not store Q values for lookup, so hard-coded rules are used (e.g. for collision, etc.)
					action := &Action{dvx: dvx, dvy: dvy}
					successor := get_successor(states, state, action)
					// By problem def, velocity components cannot both be zero.
					if successor.vx == 0 && successor.vy == 0 {
						continue
					}
					if successor.value > max_val {
						target = successor
					}
				}
			}
		}

		return target, action
	}

	// deploy worker agents to generate episodes
	episodes := make(chan *Episode, nworkers)
	agent_worker := func(
		states [][][][]State,
		start_state_gen func() *State,
		policy_fn func(*State) (*State, *Action),
		episodes chan *Episode) {

		for {
			episode := Episode{}
			state := start_state_gen()
			for !is_terminal(state) {
				successor, action := policy_fn(state)
				reward := get_reward(successor)
				episode = append(
					episode,
					Step{
						state:     state,
						successor: successor,
						action:    action,
						reward:    reward,
					})

				state = successor
			}

			episodes <- &episode
		}
	}

	for i := 0; i < nworkers; i++ {
		go agent_worker(states, rand_restart, policy_alpha_max, episodes)
	}

	processor := func(alpha, gamma float64) {
		for episode := range episodes {
			// TODO: how to set the value of the terminal state? This currently only updates values of all but the last state of an episode.
			last_step := (*episode)[len(*episode)-1]
			last_step.successor.value = last_step.reward
			//fmt.Print("Episode: ")
			// Run updates backward, such that values fully propagate back from terminal state per episode
			for _, t := range rev(len(*episode)) {
				// NOTE: not tracking states' is-visited status, so for now this is every-visit MC implementation.
				step := (*episode)[t]
				//fmt.Print("  ", step_str(&step))
				//fmt.Println("t: ", t)
				// TODO: can I show that the delta defined here will not 'fight' other updates to the same state?
				// Can this be written such that competing updates cannot interfere with the final value?
				step.state.value = step.state.value + alpha*(step.reward-gamma*step.successor.value)
			}
			//fmt.Println()
		}
	}
	go processor(alpha, gamma)
}

func step_str(step *Step) string {
	return fmt.Sprintf("%v %v %v", *step.state, *step.successor, step.reward)
}

func print_values_async(states [][][][]State) {
	for range time.Tick(time.Second * 2) {
		show_grid(states)
		show_max_values(states)
		show_policy(states)
	}
}

func main() {
	rand.Seed(time.Now().Unix())

	// choose/input a track
	racetrack := track
	// convert to state space
	states := convert_track(racetrack)
	// initialize the state values to something slightly larger than the lowest reward, for stability
	init_state_values(states, COLLISION_REWARD*2)
	// display startup policy
	show_policy(states)
	// show max values
	show_max_values(states)
	show_grid(states)

	alpha_mc_train_vanilla_parallel(states, 1)
	print_values_async(states)
}
