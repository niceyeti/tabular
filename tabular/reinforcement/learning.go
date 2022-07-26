package reinforcement

/*
This is a simple reinforcement learning application for which I implemented a few classical
RL approaches to the race track problem, and visualize the properties of the training regime in realtime
(golang runtime telemetry, value function, error, etc). The RL is purely for personal review,
not optimal implementation and behavior; these methods would be more descriptively written up in matrix
form in Python. However this implementation leverages goroutines to maximize training, albeit
modestly.
*/

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"time"

	. "tabular/grid_world"

	channerics "github.com/niceyeti/channerics/channels"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

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

type OuterConfig struct {
	Kind string      `mapstructure:"kind"`
	Def  interface{} `mapstructure:"def"`
}

// TrainingConfig is an initial stab at encoding algorithmic and training parameters outside of code.
// This definition is by no means complete or fully factored, and doesn't need to be for now, it just
// holds standard RL params like learning rates, gamma, epsilons for agent policy behavior, etc.
type TrainingConfig struct {
	// HyperParams is a key-val pair of param names and their value.
	HyperParams []HyperParameter `mapstructure:"hyperParams"`
	// Algorithm is an alg selector.
	Algorithm map[string]string `mapstructure:"algorithm"`
	// TrainingDeadline is a fixed deadline or duration describing when to terminate training.
	TrainingDeadline map[string]string `mapstructure:"trainingDeadline"`
}

type HyperParameter struct {
	Key string  `yaml:"key"`
	Val float64 `yaml:"val"`
}

func (cfg *TrainingConfig) GetHyperParamOrDefault(param string, defaultVal float64) float64 {
	for _, kvp := range cfg.HyperParams {
		if kvp.Key == param {
			return kvp.Val
		}
	}
	return defaultVal
}

// WithTrainingDeadline returns a context extended by the training deadline, if one is specified.
func (cfg *TrainingConfig) WithTrainingDeadline(
	ctx context.Context,
) (context.Context, error) {
	if val, ok := cfg.TrainingDeadline["duration"]; ok {
		if duration, err := time.ParseDuration(val); err != nil {
			return nil, err
		} else {
			innerCtx, _ := context.WithTimeout(ctx, duration)
			return innerCtx, nil
		}
	}
	// FUTURE: support a hard-deadline. I don't see the use-case, since duration works just as well.
	return ctx, nil
}

// FUTURE: a lesson learned from viper is that it doesn't seem very friendly toward multiple configs,
// though I could be wrong. For example with multiple independent config files (training, server, etc)
// viper's statefulness isn't very amenable. I could be wrong. Viper has a New() func. But I don't
// understand why config libraries (viper, flags) are not implemented as stateless functions just
// like serialization, of which config is an extension by a single degree. Also viper has quite a few
// dependencies, which is silly.
func FromYaml(path string) (*TrainingConfig, error) {
	// There was no strong reason to use viper, and app config is somewhat fragmented currently, just test driving.
	vp := viper.New()
	vp.SetConfigFile(filepath.Base(path))
	vp.SetConfigType("yaml")
	vp.AddConfigPath(filepath.Dir(path))
	var err error
	if err = vp.ReadInConfig(); err != nil {
		return nil, err
	}

	outerConfig := &OuterConfig{}
	if err = vp.Unmarshal(outerConfig); err != nil {
		return nil, err
	}

	var spec []byte
	if spec, err = yaml.Marshal(outerConfig.Def); err != nil {
		return nil, err
	}

	innerConfig := &TrainingConfig{}
	if err = yaml.Unmarshal(spec, innerConfig); err != nil {
		return nil, err
	}

	return innerConfig, nil
}

// For MC random starts, grab a random state that is on the track (i.e. is actionable to the agent).
func getRandomStartState(states [][][][]State) (start_state *State) {
	max_x := len(states)
	max_y := len(states[0])

	start_state = &states[rand.Int()%max_x][rand.Int()%max_y][0][0]
	for !(start_state.CellType == TRACK || start_state.CellType == START) {
		start_state = &states[rand.Int()%max_x][rand.Int()%max_y][0][0]
	}
	// Select a random non-zero velocity substate from this x/y position
	rvx, rvy := 0, 0
	for rvx == 0 && rvy == 0 {
		rvx = rand.Int() % NUM_VELOCITIES
		rvy = rand.Int() % NUM_VELOCITIES
	}
	start_state = &states[start_state.X][start_state.Y][rvx][rvy]
	return
}

// Gets the successor state given the domain kinematics: current position plus
// x/y velocity, plus collision constraints, equals new state.
// NOTE: the implicit kinematics here can influence the agent's learning behavior. For
// example the agent needs to know if its displacement would be along a path that would
// result in a collision; otherwise the agent will learn actions resembling 'teleports'
// to new positions. Of course rigorous check forces checking quadratic paths, but this
// will instead use simple collision checking, e.g. line-of-sight clearance from s to s'.
func getSuccessor(
	states [][][][]State,
	cur_state *State,
	action *Action,
) (successor *State) {
	// Get the proposed velocity per this Action, min of 0 and max of 4 per problem definition.
	// Though it is a little odd that the state-encoding does not encompass the action, this is
	// normal for MC, for which only state value estimates are of concern, not Q(s,a) values.
	// Logically, however, the consequence of the action *is* stored in the next state's encoding.
	new_vx := int(math.Max(math.Min(float64(cur_state.VX+action.Dvx), MAX_VELOCITY), MIN_VELOCITY))
	new_vy := int(math.Max(math.Min(float64(cur_state.VY+action.Dvy), MAX_VELOCITY), MIN_VELOCITY))
	// Get new x/y position, bounded by the grid.
	max_x := float64(len(states) - 1)
	max_y := float64(len(states[0]) - 1)
	new_x := int(math.Max(math.Min(float64(cur_state.X+new_vx), max_x), 0))
	new_y := int(math.Max(math.Min(float64(cur_state.Y+new_vy), max_y), 0))

	successor = &states[new_x][new_y][new_vx][new_vy]
	if collision := checkTerminalCollision(states, cur_state, new_vx, new_vy); collision != nil {
		successor = collision
	}

	return
}

// The collision checking algorithm is a discrete simulation of what would kinematically
// be some curving path based on the start position and velocity components. This returns
// the first terminal state encountered if starting from the passed state and proceeding
// for one time step with velocity components vx and vy. This is done by checking if the
// region spanned by start and start + (vx,vy) contains any wall cells, a hyper-conservative
// metric for collisions. Off grid actions are not accounted for.
// Returns: the first state with which the agent would collide; nil, if no collision.
func checkTerminalCollision(states [][][][]State, start *State, vx, vy int) (state *State) {
	max_x := len(states) - 1
	max_y := len(states[0]) - 1

	for dx := 0; dx <= vx; dx++ {
		newx := start.X + dx
		// Ignore out of bounds states
		if newx > max_x {
			continue
		}
		for dy := 0; dy <= vy; dy++ {
			newy := start.Y + dy
			// Ignore out of bounds states
			if newy > max_y {
				continue
			}

			traversed := &states[newx][newy][vx][vy]
			if traversed.CellType == WALL {
				state = traversed
				return
			}
		}
	}
	return
}

// Get a random velocity change (dv) in (-1,0,+1) (per problem def.).
func getRandDv() int {
	return rand.Int()%3 - 1
}

func getRandAction(cur_state *State) (action *Action) {
	action = &Action{
		Dvx: getRandDv(),
		Dvy: getRandDv(),
	}
	// By problem def velocity components cannot both be zero, so the effect of this action must be checked.
	for cur_state.VX+action.Dvx == 0 && cur_state.VY+action.Dvy == 0 {
		action.Dvx = getRandDv()
		action.Dvy = getRandDv()
	}
	return
}

func getReward(target *State) (reward float64) {
	switch target.CellType {
	case WALL:
		reward = COLLISION_REWARD
	case START, TRACK, FINISH:
		reward = STEP_REWARD
	default:
		// Degenerate case; unreachable code if all actions are covered in switch.
		panic("Shazbot!")
	}
	return
}

func is_terminal(state *State) bool {
	return state.CellType == WALL || state.CellType == FINISH
}

// For a fixed grid position, print all of its velocity subvalues.
func print_substates(states [][][][]State, x, y int) {
	fmt.Printf("Velocity vals for cell (%d,%d)\n", x, y)
	for vx := 0; vx < len(states[x][y]); vx++ {
		for vy := 0; vy < len(states[x][y][vx]); vy++ {
			s := states[x][y][vx][vy]
			val := s.Value.AtomicRead()
			fmt.Printf(" (%d,%d) %.2f\n", s.VX, s.VY, val)
		}
	}
}

// Given the current state, returns the max-valued reachable state per all available actions.
// NOTE: algorithmically the agent must consider collision when searching for the maximum
// next state. The getSuccessor function does this internally, which here results in the returned
// state presumably being a low-valued collision state (a wall). But it just needs to remembered
// that the agent's max value search must account for the environment, else its policy might converge
// to something invalid due to invalid values, by evaluating bad states as good.
func get_max_successor(states [][][][]State, cur_state *State) (target *State, action *Action) {
	maxVal := -math.MaxFloat64
	for dvx := -1; dvx < 2; dvx++ {
		for dvy := -1; dvy < 2; dvy++ {
			// Get the successor state and its value; trad MC does not store Q values for lookup, so hard-coded rules are used (e.g. for collision, etc.)
			candidate_action := &Action{Dvx: dvx, Dvy: dvy}
			successor := getSuccessor(states, cur_state, candidate_action)
			// By problem def, velocity components cannot both be zero.
			if successor.VX == 0 && successor.VY == 0 {
				continue
			}

			val := successor.Value.AtomicRead()
			if val > maxVal {
				maxVal = val
				target = successor
				action = candidate_action
			}
		}
	}
	return
}

// Train is async and initializes states and policies and begins training.
func Train(
	ctx context.Context,
	states [][][][]State,
	config *TrainingConfig,
	nworkers int,
	progressFn ProgressFunc) {
	// initialize the state values to something slightly larger than the lowest reward, for stability
	initStateVals(states, COLLISION_REWARD)
	// display startup policy
	ShowPolicy(states)
	// show max values
	ShowMaxValues(states)
	ShowGrid(states)
	alphaMonteCarloVanillaTrain(
		ctx,
		states,
		nworkers,
		config,
		progressFn)
}

func initStateVals(states [][][][]State, val float64) {
	Visit(states, func(s *State) { s.Value.AtomicSet(val) })
}

// ProgressFunc is a callback by which the training method can lend progress details,
// while exercising some level of control over its cancellation to prevent blocking.
// ProgressFunc is synchronous/blocking and should be defined to complete quickly.
type ProgressFunc func(context.Context, int)

/*
Implements vanilla alpha-MC using a fixed number of workers to generate episodes
which are sent to the estimator to update the state values. Coordination is simple:
  - agents generate and queue episodes up to some stopping criteria
  - processor halts the agents to empty its episode queue and update state values
*/
func alphaMonteCarloVanillaTrain(
	ctx context.Context,
	states [][][][]State,
	nworkers int,
	config *TrainingConfig,
	progressFn ProgressFunc) {

	// Epsilon: the agent exploration/exploitation policy param.
	epsilon := config.GetHyperParamOrDefault("epsilon", 0.1)
	// Eta: the learning rate
	eta := config.GetHyperParamOrDefault("eta", 0.01)
	// Gamma: the look-ahead parameter, or how much to value future state values.
	gamma := config.GetHyperParamOrDefault("gamma", 0.9)

	// Note: remember to exclude invalid/out-of-bound states and zero-velocity states.
	rand.Seed(time.Now().Unix())
	randRestart := func() *State {
		return getRandomStartState(states)
	}

	policyAlphaMax := func(state *State) (target *State, action *Action) {
		r := rand.Float64()
		if r <= epsilon {
			// Exploration: do something random
			action := getRandAction(state)
			target = getSuccessor(states, state, action)
		} else {
			// Exploitation: search for max-valued state per available actions.
			target, action = get_max_successor(states, state)
		}
		return target, action
	}

	// deploy worker agents to generate episodes
	agent_worker := func(
		done <-chan struct{},
		states [][][][]State,
		genInitState func() *State,
		policyFn func(*State) (*State, *Action)) <-chan *Episode {

		episodes := make(chan *Episode)
		go func() {
			defer close(episodes)

			// Generate and send episodes until cancellation.
			for {
				// done-guard
				select {
				case <-done:
					return
				default:
				}

				episode := Episode{}
				state := genInitState()
				for !is_terminal(state) {
					successor, action := policyFn(state)
					reward := getReward(successor)
					episode = append(
						episode,
						Step{
							State:     state,
							Action:    action,
							Reward:    reward,
							Successor: successor,
						})
					state = successor
				}

				select {
				case episodes <- &episode:
				case <-done:
					return
				}
			}
		}()
		return episodes
	}

	// Fan in the workers to a single channel. This allows the processor to throttle the agents
	// by not pulling episodes from their chans, which in turn pseudo-serializes matrix read/write.
	// Note: the serialization is not robust or production worthy sans locking the state matrix.
	// Chans provide a sufficient coordination mechanism for prototyping, but is not rigorous (e.g.
	// will fail builds with '-race' flag).
	// TODO: locking algorithms or strategies for large resource space, where every item in the space
	// feasibly requires a lock?
	workers := []<-chan *Episode{}
	for i := 0; i < nworkers; i++ {
		ch := agent_worker(ctx.Done(), states, randRestart, policyAlphaMax)
		workers = append(workers, ch)
	}
	episodes := channerics.Merge(ctx.Done(), workers...)

	// Estimator updates state values from agent experiences.
	estimator := func(
		eta, gamma float64,
		progressFn ProgressFunc) {
		episode_count := 0
		for episode := range episodes {
			// Set terminal states to the value of the reward for stepping into them.
			last_step := (*episode)[len(*episode)-1]
			last_step.Successor.Value.AtomicSet(last_step.Reward)
			// Propagate rewards backward from terminal state per episode
			reward := 0.0
			for _, t := range Rev(len(*episode)) {
				// NOTE: not tracking states' is-visited status, so for now this is an every-visit MC implementation.
				step := (*episode)[t]
				reward += step.Reward
				val := step.State.Value.AtomicRead()
				delta := eta * (reward - val)
				// Note: intentionally discard rejected deltas. There won't be any, since add ops are serialized
				// as there is a single estimator.
				_, _ = step.State.Value.AtomicAdd(delta)
			}

			// Hook: periodically do some other processing (publishing state values for views, etc.)
			episode_count++
			progressFn(ctx, episode_count)
		}
	}
	go estimator(eta, gamma, progressFn)
}
