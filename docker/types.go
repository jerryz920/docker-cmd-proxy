package docker

type PortRange struct {
	min int
	max int
}

type PortAlias struct {
	min      int
	max      int
	protocol string
	ip       string
	nsName   string
}
