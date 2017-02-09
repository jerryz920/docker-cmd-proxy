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

	containerPath = config.Daemon.ContainerPath
	imagePath = config.Daemon.ImagePath
	if len(args) >= 1 {
		containerPath = args[0]
	}
	if len(args) >= 2 {
		imagePath = args[1]
	}

	monitor := daemon.InitMonitor(containerPath, imagePath)
	defer monitor.Close()

	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGUSR1, syscall.SIGUSR2)

	done := make(chan bool)
	go func() {
		monitor.WaitForEvent(sigchan)
	}()

	<-done
}
