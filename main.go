// watchrun is a small tool for continuously running and restarting a target
// binary. Whenever the binary changes, watchrun kills the running old binary
// and starts a copy of the new instance. This works well with an editor that
// supports running "go install" (such as with vim-go).

package main

import (
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

type signaler struct {
	mu sync.Mutex
	ch chan struct{}
}

func newSignaler() *signaler {
	return &signaler{
		ch: make(chan struct{}),
	}
}

func (s *signaler) signal() {
	s.mu.Lock()
	defer s.mu.Unlock()

	close(s.ch)
	s.ch = make(chan struct{})
}

func (s *signaler) wait() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ch
}

func watchFile(path string) (*signaler, error) {
	signaler := newSignaler()

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	go func() {
		modTime := info.ModTime()

		for {
			time.Sleep(1 * time.Second)

			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				panic(err)
			}

			newModTime := info.ModTime()
			if !modTime.Equal(newModTime) {
				signaler.signal()
				modTime = newModTime
			}
		}
	}()

	return signaler, nil
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("usage: %s binary <args>", os.Args[1])
		os.Exit(1)
	}

	binary, err := exec.LookPath(os.Args[1])
	if err != nil {
		log.Printf("failed to find binary: %s\n", err)
		os.Exit(1)
	}
	args := os.Args[2:]

	signaler, err := watchFile(binary)
	if err != nil {
		log.Printf("failed to watch binary: %s\n", err)
		os.Exit(1)
	}

	for {
		changed := signaler.wait()

		cmd := exec.Command(binary, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		done := make(chan struct{}, 0)
		go func() {
			select {
			case <-done:
				break
			case <-changed:
				cmd.Process.Kill()
				break
			}
		}()

		log.Printf("(re-)starting %s", binary)
		if err := cmd.Start(); err != nil {
			log.Printf("failed to start binary: %s\n", err)
			os.Exit(1)
		}

		cmd.Wait()
		close(done)
	}
}
