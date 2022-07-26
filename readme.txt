This is a review/kata project for reinforcement learning in Golang:
* review Golang, flex goroutines for parallel rl-training
* reimplement some old RL environments and test beds

Tasks:
1) Grid world interfaces for the race car problem, amenable to parallelization
2) alpha-MC: on an off-policy learning
3) Q-learning 
4) Parallize them
5) Design a front-end to show how the value function changes during training (use Gotemplates)

The primary environmental basis for this mini project is the racetrack problem detailed in Sutton's
Reinforcement Learning, exercise 5.4.

# Software design

## Models
- Value function
- States and actions
- Transition model
- Policies
* For now, I am going to assume a deterministic discrete environment: action a in state s yields s' with P((s,a) -> s') = 1.0.
  Also, the environment is not the general environment of driving; this is much simpler, for review and interchangeable methods.
  The track is a fixed curve where each state maps to a susbtate of this specific problem instance, not the general driving problem.
  Note this isn't actually much different than memorizing a course in a game or a real life race course.
  Nor have I made any attempt to simplify/batchify the math using linear algebra (e.g. for batch training), merely agent based one-step learning.
Example:
"In state s, take action a, proceed to s' with reward r."

## Tasks
- Initialize world, value function, parallel agents
- Train
- Display realtime learned policy, value function, other problem data to observe learning/stability

## Specifics
Given this world, where 'W' denotes a wall, 'o' the track, '-' a starting state, '+' a finish state:
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

The action space consists of the agent increasing, decreasing, or leaving-alone its current velocity by +1, -1, or 0, respectively.
Thus the state space are each position (x,y) shown above, plus the agent's current velocity (x,y,v).
The action space is increasing or decreasing its current velocity, delta-V:
The agent cannot increase its velocity above/below 5/0.
The rewards are -1 for each time step and -5 if it crashes.

For simplicity and debugging, a smaller course is this:
tiny_course = ['WWWWWW',
               'Woooo+',
               'Woooo+',
               'WooWWW',
               'WooWWW',
               'WooWWW',
               'WooWWW',
               'W--WWW',]

Sources:
* racetrack example and instances: https://gist.github.com/pat-coady/26fafa10b4d14234bfde0bb58277786d

func main() {





}




Frontending:

The goal is simply this: much like generating an svg describing the current value function 
and policy using go templates (which would be super easy), I simply want to ensure such
a view dynamically updates:
    1) Navigating to the page causes the visualization to refresh by default
    2) Some compact description language for communicating what should change and how
    Update function:
        - Given ele id, update these attributes keys with these values
    Thus a state has-a rendering function encapsulating one target view type (svg ele, xml ele, etc):
    - Id() string: the ele id
    - String() string: svg ele string (e.g. rect, arrow, etc.) used to initialize the element as html
    - Update() (key string, val string)[]: convert a value change to a set of new attribute key/values
    The socket listens for these updates. When it receives them,
    it runs a simple procedural loop in vanilla js:
        for ele_delta in update:
            dom_ele = dom.Get(ele_delta.id)
            for kvp in ele_delta:
                dom_ele.attrib[kvp.key] = kvp.value
    The svg will automatically render the new values.




- efficient, fast to produce, something a research coder could build rapidly, not over-speced
- prefer svg because html elements are used (and html events), unlike canvas
- server side push (websockets)
- golang generated html template
- some default communication glue (websockets perfect for this)
- focused solely on a pure, simple function: given these values changes, update these items
    * divide and conquer data from ui
    * transparent mapping of some kind
    * could use element ids to store positional/relational info for updating the ele

Multiple solutions:
1) Golang generated template and websockets
    * generate the svg once, containing ele id info (for position, type, etc)
    * on server side change, notify, send message: each ele knows a function for updating itself
    * svg attributes provides a somewhat compact static language; it should be possible to translate go-code to these changes
    * 

    "Given this ele id, 1) replace existing 2) update its attributes"


The core question is when X changes on the server, what ele should be updated and how should it be updated?
Doing this in a compact transparent manner.



















