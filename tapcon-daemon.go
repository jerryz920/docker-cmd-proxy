package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	config "github.com/jerryz920/tapcon-monitor/config"
	daemon "github.com/jerryz920/tapcon-monitor/docker"
)

const (
	pidFile = "/var/run/tapcon.pid"
)

func daemonize() {
	/// write pid

}

func main() {
	flag.Parse()
	args := flag.Args()
	config.InitConf(".")

	containerRoot := conf.Daemon.ContainerRoot
	if len(args) >= 1 {
		containerRoot = args[0]
	}
	log.Printf("container root: %s\n", containerRoot)

	monitor, err := daemon.NewMonitor(containerRoot, nil, nil)
	if err != nil {
		log.Fatalf("error allocating new monitor:%s\n", err)
	}
	monitor.Dump()

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGUSR1, syscall.SIGUSR2)

	done := make(chan bool)
	go func() {
		monitor.WorkAndWait(sigchan)
	}()
	<-done
}
