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






















