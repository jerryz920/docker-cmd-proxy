package docker

import (
	"testing"
	"time"
)

func TestLoadContainer(t *testing.T) {

	id := "b3f37be527fa2e44d4916497d22ed635d21e10d0b991682939365c0e6b1f5101"
	if container, err := LoadContainer(id,
		"../tests/containers/"+id, time.Now(), true); err != nil {
		t.Fatalf("error occurs: %s\n", err.Error())
	} else {
		t.Logf("container config %v", container.Config.Hostname)
	}

}
