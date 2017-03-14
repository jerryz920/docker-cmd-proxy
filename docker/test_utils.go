package docker

import (
	"os/exec"
	"path/filepath"

	config "github.com/jerryz920/tapcon-monitor/config"
)

const (
	scriptDir = "../tools/tests/"
)

func AddContainer(id string, dead, docker bool) error {
	cmdPath := filepath.Join(scriptDir, "copy_containers.sh")
	if docker {
		cmdPath = filepath.Join(scriptDir, "docker_make_containers.sh")
	}

	target := "alive"
	if dead {
		target = "dead"
	}
	return exec.Command(cmdPath, target, id).Run()
}

func DelContainer(id string) error {
	cmdPath := filepath.Join(scriptDir, "rm_container.sh")
	return exec.Command(cmdPath, id).Run()
}

func RmAllTestContainers() error {
	cmdPath := filepath.Join(scriptDir, "rmall.sh")
	return exec.Command(cmdPath).Run()
}

func initTestConfig() {
	config.InitConf("../tests/")
}

func StubListIP(ns string) []string {
	return []string{"192.168.1.1"}
}
