/*
This is a single page reinforcement learning application for which I implemented a few classical
RL approaches to the race track problem, and visualize the properties of the training regime in realtime
(golang runtime telemetry, value function, error, etc). The RL is purely for personal review,
not optimal implementation and behavior; these methods would be more descriptively written up in matrix
form in Python. However this implementation leverages goroutines to maximize training, albeit
modestly.
*/

package main

import (
	"context"

	. "tabular/models"
	. "tabular/reinforcement"
	. "tabular/server"
)

var (
	state_snapshots chan [][][][]State = make(chan [][][][]State, 0)
	states          [][][][]State
)

func run() {
	// choose/input a track
	racetrack := Debug_track
	// convert to state space
	states = Convert(racetrack)
	Train(
		states,
		export_states,
		context.TODO())
	// TODO: read and pass in the addr and port
	NewServer(
		states,
		state_snapshots,
	).Serve()
}

// When called during training progress, this blocks and sends the current
// state values to the server to update views.
func export_states(episode_count int, done <-chan struct{}) {
	if episode_count%100 == 1 {
		select {
		case state_snapshots <- states:
		case <-done:
		}
	}
	return
}

/*
func print_values_async(states [][][][]State, done <-chan struct{}) {
	for range channerics.NewTicker(done, time.Second*2) {
		show_grid(states)
		show_max_values(states)
		show_avg_values(states)
		show_policy(states)
		//print_substates(states, 9, 4)
	}
}
*/

func main() {
	// read the --debug flag, others
	// start training
	// start server
	run()
}
