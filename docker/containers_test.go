package docker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	docker "github.com/docker/docker/container"
	"github.com/jerryz920/tapcon-monitor/config"
	"github.com/stretchr/testify/assert"
)

func init() {
	config.InitConf("..")
}

func TestMemContainerOutofDate(t *testing.T) {
	id := "b3f37be527fa2e44d4916497d22ed635d21e10d0b991682939365c0e6b1f5101"
	path, err := filepath.Abs("../tests/backups/")
	if err != nil {
		t.Fatalf("error in container root conversion: %v\n", err)
	}
	root := filepath.Join(path, id)
	c := NewMemContainer(tapconStringId(id), root, "")

	config := filepath.Join(root, "config.v2.json")
	hostconfig := filepath.Join(root, "hostconfig.json")

	assert.True(t, c.OutOfDate(), "initialized container should be out of date")

	c.LastUpdate = time.Now()
	baseContainer := docker.NewBaseContainer(c.Id, c.Root)
	c.Config = baseContainer

	assert.False(t, c.OutOfDate(), "updated container should not be out of date")

	time.Sleep(10 * time.Microsecond)
	now := time.Now()
	if err := os.Chtimes(config, now, now); err != nil {
		t.Fatalf("can not update file %s's mod time\n", config)
	}
	assert.True(t, c.OutOfDate(), "config changed")

	c.LastUpdate = time.Now()
	time.Sleep(10 * time.Microsecond)

	now = time.Now()
	if err := os.Chtimes(hostconfig, now, now); err != nil {
		t.Fatalf("can not update file %s's mod time\n", config)
	}
	assert.True(t, c.OutOfDate(), "config changed")
}

func TestMemContainerLoad(t *testing.T) {
	id := "b3f37be527fa2e44d4916497d22ed635d21e10d0b991682939365c0e6b1f5101"
	path, err := filepath.Abs("../tests/backups/")
	if err != nil {
		t.Fatalf("error in container root conversion: %v\n", err)
	}
	root := filepath.Join(path, id)
	c := NewMemContainer(tapconStringId(id), root, "")
	c.listIp = StubListIP

	assert.True(t, c.Load(), "c contains valid state")
	assert.Equal(t, c.Ips, []string{"192.168.1.1"})
	assert.True(t, c.Load(), "c contains valid state even if load again where outofdate works")

	config := filepath.Join(root, "config.v2.json")
	now := time.Now()
	if err := os.Chtimes(config, now, now); err != nil {
		t.Fatalf("can not update file %s's mod time\n", config)
	}
	assert.True(t, c.Load(), "load changed container")
}
