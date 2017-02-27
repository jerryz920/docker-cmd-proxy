package docker

const (
	TestDir = "../tmp/containers"
)

func CopyContainerDir(src, dst string) {

}

func AddContainer(id string, dead, copy bool) {
	var targetName string

	if dead {
		targetName = "dead"
	} else {
		targetName = "alive"
	}

}

func TestContainerEntriesUpdate() {

}
