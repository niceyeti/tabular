To build or update the dev container, do the following in .devcontainer:
1)  Rebuild the base container and tag it `godev:latest`:
    * `docker build -f base.Dockerfile -t godev:latest .`
2) Review Dockerfile; it points to godev:latest. Also review .devcontainer.json,
   which points to Dockerfile.
3) Open vscode and select "Rebuild container". This
   will rebuild the vscode container itself. It may hang after building; if so,
   reopening vscode worked previously.

The setup itself is derived from Microsoft's devcontainer go examples, which are a PITA to interpret.
When golang 1.18 and Microsoft publishes a new official go-1.18 vscode image,
it may be best to update to that. However I like knowing how my stuff works, so probably just
review and rebuild from scratch for the experience.

TODO:
1) note poor security posture of the dev container, which use seccomp=unconfined and adds ptrace capability. This is no doubt for debugging, "ease of use", but be aware of the settings and update them if possible.
2) update docker image to golang 1.18 image when released



