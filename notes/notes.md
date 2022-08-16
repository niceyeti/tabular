Interesting sub-problems from this project:

1) Efficient locking for a large data structure, e.g. size m, with n workers,
and m >> n. See parallelism doc.
2) Auto-generated view components: the goal was to generate realtime views of the RL
algorithm performance/training without a big ugly js framework. As shown, this can be done
using the html/template package, but I did not abstract the entities and their work.
For example, one approach might be to 
3) Similar to (2), I would like a means of federating multiple view components. Of course, the same
goal of bare bones simplicity applies: the end goal is a reusable library, not a framework. The problem
definition is that I want to implement components that consume state information and translate them into
view components (e.g. svg), as well as definitions by which to update those components.
4) Overall view-simplification: Every view performs the same job, which is: given some state information
(published by the training algorithm) implements a series of transformations to that info and output
a chunk of svg. So when you zoom out from the task of implementing these views, you notice that 
a developer handed such a task ("Okay dev, now make me a values chart") merely implements this
transformation from data to views. I'm just curious how general you could make such a solution.
Recall that in the extreme abstraction, you could implement essentially a server-side virtual dom:
1) each component generates its html to add to the initial html of the page 2) when data are updated
they apply changes to this dom 3) some other component diffs the changes and pushes only these to the client.
    * (3) makes the approach fully generic, though it is functionally equivalent to letting the
      components generates publish their updates operations to be applied on the client.
    * Is there an existing such golang project?
    * 











DevSecOps with AH:
1) His background in DevSecOps
Research projects:
- diodes etc
- any current open source / projects I can grab?
- Ohio life
- MITRE lifestyle and role structure:
    * dependent on government grants?
    * longevity of work seems strong, security will not cease relevance
    * 
3) Research connections at WSU: I am completely willing to do pro bono security work
while I search for a new job. Not only will it sharpen my skills, I think it would be
extremely fun. The independence would be awesome compared with my recent experience at
SEL (not bad, but they optimize for your time as they ought to).

My experience at SEL hardened my interest DevSecOps. It solves valuable problems,
and I'm somewhat uncompromising when it comes to finding a role in that space.
While I still love and envy ML work, DevSecOps is hyper-focused on developer productivity,
teamwork, and leadership, that I'm grateful to have found a real niche where I can lose
myself in work.

Personal/Seattle stories: probably let Adam drive these. SEL would have me back, but I've
outgrown them (anti-OSS, framework obsessed, anti-linux when building linux platforms).

AH:
- Colorado Springs is a go
- Lab work is given at leisure, researchers choose projects to work on
- Could follow up per MITRE roles
- Next directions: NAVSEA, PNNL Seattle, etc. Giddy up and go.




