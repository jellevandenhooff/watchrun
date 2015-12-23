// watchrun is a small tool for continuously running and restarting a target
// binary. Whenever the binary changes, watchrun kills the running old binary
// and starts a copy of the new instance. This works well with an editor that
// supports running "go install" (such as with vim-go).

package main

import (
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/jellevandenhooff/concurrency"
)

func lookupBinary(name string) (string, time.Time, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", time.Time{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}, err
	}

	return path, info.ModTime(), nil
}

func watchBinary(name string) *concurrency.Signaler {
	signaler := concurrency.NewSignaler()

	lastPath, lastModTime, _ := lookupBinary(name)

	go func() {
		for {
			time.Sleep(1 * time.Second)

			path, modTime, err := lookupBinary(name)
			if err != nil {
				continue
			}

			if path != lastPath || modTime != lastModTime {
				signaler.Signal()
				lastPath, lastModTime = path, modTime
			}
		}
	}()

	return signaler
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("usage: %s binary <args>", os.Args[1])
		os.Exit(1)
	}

	binary := os.Args[1]
	args := os.Args[2:]

	signaler := watchBinary(binary)

	for {
		changed := signaler.Wait()

		cmd := exec.Command(binary, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		log.Printf("Starting %s", binary)
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to start binary: %s\n", err)

			select {
			case <-changed:
				log.Printf("Binary changed; continuing")
			}
			continue
		}

		done := make(chan struct{}, 0)
		go func() {
			select {
			case <-done:
				break
			case <-changed:
				log.Printf("Binary changed; killing running process")
				cmd.Process.Kill()
				break
			}
		}()
		cmd.Wait()
		close(done)
	}
}
