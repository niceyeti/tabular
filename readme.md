# Tabular

Although tabular RL methods are often used to introduce the general structure of RL problems, they are still highly expressive, mathematically robust, and worthy of a nice implementation. I needed some language and app dev review, and pulled the classic [Sutton and Barto](http://incompleteideas.net/book/the-book-2nd.html) off the shelf as the target domain for doing so.

Tabular is an app that implements and visualizes tabular RL methods in golang. The backend uses [generic-channel patterns](https://github.com/niceyeti/channerics) to parallelize training; the frontend is also 100% golang, leveraging go-templates and websockets to rapidly develop server-generated/server-push based frontends describing the performance and training of RL implementations. Being template-generated and websocket-updated, the frontend components are essentially readonly visualizations that an RL developer would use to observe the characteristics of an algorithm or its parameters, without the overhead of a js-framework.

A gif of one of the front-end components, a value-function surface plot, lends some intuition:

![Monte Carlo Value-Function Approximation](./docs/monte_carlo_fn_surface.gif)

The surface is an isometric projection of the value function for the textbook 'racetrack' problem. The contours of the value function for a racetrack corner emerge as the concurrent agents learn the value of states in the 2D coordinate system (the states actually comprise a higher number of kinematic dimensions, per x/y velocities). Blue represents higher value and red represents lower value.

A key reason for visualizing value-functions is highlighted by the emergence of the light-red valley along the top-left diagonal. Given the reward structure, the choice of hyper-parameters, and the selected algorithm (MC-alpha), 
the agent finds itself, morbidly, in "suicide valley". Along these state trajectories the agent determines that maximum reward is obtained by crashing off-track, too far from the goal to obtain positive value. The radius of possible goal-obtaining behavior is shown by the blue/purple steppes of width ~4: four units per timestep is the agent's maximum velocity, thus with a punishment of -1 per timestep, only states within four timesteps exceed that of the off-course -5 states. The arithmetic details aren't important; the example is purely demonstrative, as summary performance metrics would not have caught the defective behavior of the policy. Re-engineering the reward structure or swapping the algorithm would result in a more optimistic agent.

But FWIW, stay out of any self-driving Muskwagen for which there is not comprehensive value function documentation.

## Project Organization

* reinforcement/: here lies code for the domain. Compile-time hyper-parameters, because awaiting recompiles gives you that warm 'I'm working' feeling!
* atomic_float/: a package for atomic float ops, or "How I cheated my way out of proper matrix locks using atomic ops". I am still considering alternatives to solve the general problem of multiple workers for large matrices.
* server/.../fastview: this is a first-crack at declarative front-end components, a learning experience in go-templates. Loosely, each view entails:

    1) some view-model derived from the state models and a conversion function thereof
    2) a declarative go-lang template describing the component's initial structure 
    3) a channel for sending updates to target eles in the client dom

## Core goals

0) Re-learn some stuff, have fun
1) Clean, end-to-end observability patterns: data updates should automatically trigger ui updates, end-to-end. This can be achieved easily using event-based programming, but the goal is declarative views with linq-like business logic that is easy to read, share, and maintain.
2) App organization: review uncle bob

## Future considerations

0) Server-side virtual dom: this has to have been done elsewhere. Desirably this: arbitrary server-side view components update a server-side dom **D1**, a diffing engine detects these changes, and sends exactly and only these changes to the client. The diffing layer completely decouples view components from update logic; the current code requires them to implement their own updates as EleUpdate's. The diffing layer acts as a throttling mechanism and has some other benefits.
1) CSP-style concurrent matrix modification: n workers operate on a matrix (or any arbitrary large data structure) of size m, where n << m. I cheated my way out of locks using atomic ops, but it dirties the code (I still need to encapsulate it all in an AtomicFloat type). Lock options include:
    1) Ask others for ideas for how to use n locks, rather than m (flyweight for locks?)
    2) Optimistic locking: or whatever locking mechanism K8s uses, where workers increment a counter, update an object, then check if the counter changed. If so, discard the update, else, perform the patch, etc.
