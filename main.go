// watchrun is a small tool for continuously running and restarting a target
// binary. Whenever the binary changes, watchrun kills the running old binary
// and starts a copy of the new instance. This works well with an editor that
// supports running "go install" (such as with vim-go).

package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

func kill(cmd *exec.Cmd) {
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		log.Fatal(err)
	}
	syscall.Kill(-pgid, 9)
}

func main() {
	if len(os.Args) < 2 {
		log.Printf("usage: %s binary <args>", os.Args[1])
		os.Exit(1)
	}

	binary := os.Args[1]
	args := os.Args[2:]

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	signaler := watchBinary(binary)

	backoff := 1 * time.Second

	for {
		changed := signaler.Wait()

		cmd := exec.Command(binary, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
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
			case <-changed:
				log.Printf("Binary changed; killing running process")
				kill(cmd)
			case <-interrupt:
				kill(cmd)
				os.Exit(0)
			}
		}()
		cmd.Wait()
		select {
		case <-changed:
			backoff = 1 * time.Second
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
		close(done)
	}
}
