package docker

import (
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	metadata "github.com/jerryz920/tapcon-monitor/statement"
	"github.com/stretchr/testify/assert"
)

func init() {
	initTestConfig()
}

func newStubMonitor(t *testing.T) *Monitor {
	m, err := NewMonitor("../tests", metadata.NewStubApi(t), &fakeSandbox{})
	if err != nil {
		t.Fatalf("can not allocate monitor %v\n", err)
	}
	return m
}

func PrepareInitialContainers(t *testing.T) {

	if err := AddContainer("c1", true, false); err != nil {
		t.Fatalf("fail to add container %v\n", err)
	}
	AddContainer("c2", true, true)
	AddContainer("c3", true, false)
	AddContainer("c4", true, true)
}

func TestContainerEntriesUpdate(t *testing.T) {
	m := newStubMonitor(t)
	sigchan := make(chan os.Signal)
	signal.Notify(sigchan, syscall.SIGUSR1, syscall.SIGUSR2)
	defer RmAllTestContainers()

	PrepareInitialContainers(t)
	go m.WorkAndWait(sigchan)
	time.Sleep(2 * time.Second)

	m.ContainerLock.Lock()
	assert.Len(t, m.Containers, 4, "loaded containers")
	m.ContainerLock.Unlock()

	AddContainer("c5", true, false)
	AddContainer("c6", true, true)
	AddContainer("c7", true, false)

	time.Sleep(2 * time.Second)
	m.ContainerLock.Lock()
	assert.Len(t, m.Containers, 7, "dyn adding containers")
	for _, c := range m.Containers {
		assert.False(t, c.Running(), "dead containers")
	}
	m.ContainerLock.Unlock()

	DelContainer("c1")
	DelContainer("c2")
	DelContainer("c3")

	time.Sleep(2 * time.Second)
	m.ContainerLock.Lock()
	assert.Len(t, m.Containers, 4, "dyn removing containers")
	m.ContainerLock.Unlock()

	AddContainer("t1", false, false)
	AddContainer("t2", false, true)
	AddContainer("t3", false, false)
	AddContainer("t4", false, true)

	time.Sleep(2 * time.Second)
	m.ContainerLock.Lock()
	assert.Len(t, m.Containers, 8, "dyn adding alive containers")
	for id, c := range m.Containers {
		if id == "t1" {
			assert.True(t, c.Running(), "t1 is running")
		}
	}
	m.ContainerLock.Unlock()
	DelContainer("t1")
	DelContainer("t2")

	time.Sleep(2 * time.Second)
	m.ContainerLock.Lock()
	assert.Len(t, m.Containers, 6, "dyn adding alive containers")
	m.ContainerLock.Unlock()

}
