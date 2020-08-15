package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

var reaperErrLog *log.Logger

func init() {
	reaperErrLog = log.New(os.Stderr, "reaper: ", log.LstdFlags|log.Lmicroseconds)
}

func reapChildProcesses() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGCHLD)

	for {
		// Block until SIGCHLD signal is received.
		<-c
		reapChildProcessesHelper()
	}
}

func reapChildProcessesHelper() {
	for {
		// Pid -1 means to wait for any child process.
		// With syscall.WNOHANG option set, function will
		// not block and will exit immediately if no child
		// process has exited.
		pid, err := syscall.Wait4(-1, nil, syscall.WNOHANG, nil)
		switch err {
		case nil:
			// If pid == 0 then one or more child processes still exist,
			// but have not yet changed state so we return and wait
			// for another SIGCHLD.
			if pid == 0 {
				return
			}
		case syscall.ECHILD:
			// No more child processes to reap. We can return and wait
			// for another SIGCHLD signal.
			return
		case syscall.EINTR:
			// Got interrupted. Shouldn't happen with WNOHANG option,
			// but it is better to handle it anyway and try again.
		default:
			// Some other unexpected error. Return and wait for
			// another SIGCHLD signal.
			reaperErrLog.Printf("Unexpected error waiting for child process: %v\n", err)
			return
		}
	}
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

	if os.Getpid() == 1 {
		log.Println("Starting reaper")
		go reapChildProcesses()
	}

	// Block until a SIGINT, SIGKILL or SIGTERM signal is received.
	<-c
}
