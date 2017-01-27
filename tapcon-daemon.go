package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	config "github.com/jerryz920/tapcon-monitor/config"
	daemon "github.com/jerryz920/tapcon-monitor/daemon"
)

func main() {
	flag.Parse()
	args := flag.Args()
	config.InitConf(".")

	monitor := daemon.InitMonitor(args[0])
	defer monitor.Close()

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGUSR1, syscall.SIGUSR2)

	done := make(chan bool)
	go func() {
		monitor.WaitForEvent(sigchan)
	}()

	<-done
}
