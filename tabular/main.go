/*
This is a single page reinforcement learning application for which I implemented a few classical
RL approaches to the race track problem, and visualize the properties of the training regime in realtime
(golang runtime telemetry, value function, errors, etc). The RL is purely for personal review,
not optimal implementation and behavior; these methods would be more descriptively written up in matrix
form in Python. However this implementation leverages goroutines to maximize training, albeit
modestly. RL is somewhat disagreeable toward code abstraction because coding pretty textbook
algorithms always yields edgecases and a mixture of logical and mathematical issues. IOW,
don't think too hard and go with procedural solutions. The goal is simply to show methods operating
correctly, not formal research.
*/

package main

import (
	"context"
	"flag"
	"fmt"
	"runtime"

	"tabular/grid_world"
	"tabular/reinforcement"
	"tabular/server"
)

var (
	stateUpdates chan [][][][]grid_world.State = make(chan [][][][]grid_world.State)
	states       [][][][]grid_world.State
	dbg          *bool
	nworkers     *int
	host         *string
	port         *string
	addr         string
)

func init() {
	dbg = flag.Bool("debug", false, "debug mode")
	nworkers = flag.Int("nworkers", runtime.NumCPU(), "number of worker training routines")
	host = flag.String("host", "", "The host ip")
	port = flag.String("port", "8080", "The host port")
	addr = *host + ":" + *port
	flag.Parse()
}

func selectTrack() []string {
	// choose/input a track
	if *dbg {
		return grid_world.DebugTrack
	}
	return grid_world.FullTrack
}

func runApp() (err error) {
	// TODO: I'm not super worried about setting up elegant teardown. It would
	// be a good exercise. The contexts are not super clear either. The gist is
	// that rootCtx could represent a shutdown signal, etc., but usage is not needful.
	appCtx, appCancel := context.WithCancel(context.TODO())
	defer appCancel()

	racetrack := selectTrack()
	states = grid_world.Convert(racetrack)

	// Start training
	reinforcement.Train(
		appCtx,
		states,
		*nworkers,
		exportStates)

	// Run server
	var srv *server.Server
	if srv, err = server.NewServer(
		appCtx,
		addr,
		states,
		stateUpdates,
	); err != nil {
		return
	}

	err = srv.Serve()
	return
}

// When called during training progress, this blocks and sends the current
// state values to the server to update views.
func exportStates(episode_count int, done <-chan struct{}) {
	if episode_count%1000 == 1 {
		select {
		case stateUpdates <- states:
		case <-done:
		}
	}
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

// TODO: use mixedCaps throughout
func main() {
	if err := runApp(); err != nil {
		fmt.Println(err)
	}
}
