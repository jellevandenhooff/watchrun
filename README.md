# watchrun

1. `go install github.com/jellevandenhooff/watchrun`
2. `watchrun myproject`
3. `go install ./myproject`

From now, `watchrun` will keep restarting `watchrun` whenever you run `go
install` and the binary changes.
