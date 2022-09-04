/*
Tabular is a single page reinforcement learning application for which I implemented a few classical
RL approaches to the race track problem, and visualize the properties of the training regime in realtime
(golang runtime telemetry, value function, errors, etc). The RL is purely for personal review,
not optimal implementation and behavior; these methods would be more descriptively written up in matrix
form in Python. However this implementation leverages goroutines to maximize training, albeit
modestly. RL is somewhat disagreeable toward code abstraction because coding their pretty textbook
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

/*
Reactive algorithms? Good for dedicated learning pipelines in the cloud...
- Load new problem instances
- Load different hyper-parameters
- Scale vertically and horizontally: useful for resources but also to span host networks
- Monitor parameters or other properties and update only what is required, rather than restarting from scratch.
For example: using Viper you can monitor local and remote config sources for change, and execute
actions when they occur. This is a simple combination of the Observer pattern and ML development workflows.
It basically resolves to a reactive builder pattern, per usual:
	// Returns a complete observable computational graph of input components downstream components are rebuilt if any change.
	alg := NewTrainBuilder()
		.WithDataset("census.txt")
		.WithKey(&dataDecryptionKey)
		.WithHyperParams(&hypers)
		.WithStopCondition(stopFn)  // When to halt training
		// horizontal scale params
		.WithDeadline(time.Minute * 10)
		.WithBudget(&computeBudget)
		// vertical scale params
		.WithInstances()
		.WithProgressEndpoint("http://10.10.1.200/my-alg-progress")
		.Build()

	srv := NewServer()
		.WithCert()
		.WithViews(...)

ML functions are similar in representation to sensors, so its fun to think about clustered-IoT
constructs also: implement a builder incorporating quality checks, etc, and integrating data
streams of various kinds, transforming them, exporting, etc.
	.Select(func(host string) bool { return strings.StartsWith(host, "rpi") })
	.WithSensor(&led)
	.WithSensorLivenessCheck()
	.WithTransformer(convertFunc) // transform signal into another form
	.WithExporter() // Somewhere to export data to, once
	.WithBlahBlah(...)
In this manner, write easy-to-use interfaces to build entire sensor networks, for which logging,
health checks, security, auditability, and reversibility (helm charts) are built in. Ignore the
fact that the above conflates different models of things (hosts, sensors, etc) and doesn't model
them properly; each distinct entity (a sensor, a view, a dataset) could be modeled as an MVC.
There's also the matter/potential for a declarative specification of each layer. Also, completely
ignoring legacy device/protocol crud... no need for settings/protocol monster projects. Just focus
on dirt simple rpi projects: discrete input/output sensors, video streams, etc, for security or
other small, rapidly developed applications.
*/

// TODO: per 12-factor rules, these should be taken from env or config-map; KISS for now. Also init is bad.
func init() {
	dbg = flag.Bool("debug", false, "debug mode")
	nworkers = flag.Int("nworkers", runtime.NumCPU(), "number of worker training routines")
	host = flag.String("host", "", "The host ip")
	port = flag.String("port", "8080", "The host port")
	addr = *host + ":" + *port
	flag.Parse()
}

func selectTrack() []string {
	// choose or input a track
	if *dbg {
		return grid_world.DebugTrack
	}
	return grid_world.FullTrack
}

func runApp() (err error) {
	var algConfig *reinforcement.TrainingConfig
	if algConfig, err = reinforcement.FromYaml("./config.yaml"); err != nil {
		return
	}

	appCtx, appCancel := context.WithCancel(context.TODO())
	defer appCancel()

	trainingCtx, _ := algConfig.WithTrainingDeadline(appCtx)

	racetrack := selectTrack()
	states = grid_world.Convert(racetrack)

	// Start training
	reinforcement.Train(
		trainingCtx,
		states,
		algConfig,
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
func exportStates(ctx context.Context, episodeCount int) {
	if episodeCount%1000 == 1 {
		select {
		case stateUpdates <- states:
		case <-ctx.Done():
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
