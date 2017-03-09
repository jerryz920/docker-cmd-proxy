package docker

import (
	"fmt"
	"reflect"
	"testing"

	container_types "github.com/docker/docker/api/types/container"
	api_types "github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/container"

	log "github.com/Sirupsen/logrus"
	docker_network "github.com/docker/docker/daemon/network"
	docker_image "github.com/docker/docker/image"
	nat "github.com/docker/go-connections/nat"
	metadata "github.com/jerryz920/tapcon-monitor/statement"
	assert "github.com/stretchr/testify/require"
)

func newStubContainer(id, image, publicip, localip, localns string,
	overlayIp, overlayId string,
	staticPortMin, staticPortMax int, exportedPorts ...int) *MemContainer {

	c := &MemContainer{
		Config: &docker.Container{},
	}
	c.Config.NetworkSettings = &docker_network.Settings{
		Networks: make(map[string]*docker_network.EndpointSettings),
		Ports:    nat.PortMap{},
	}
	c.Config.HostConfig = &container_types.HostConfig{}
	c.Config.HostConfig.NetworkMode = "userdefined"
	c.listIp = StubListIP

	//
	t := reflect.TypeOf(c.Config.NetworkSettings.Ports)
	c.Config.NetworkSettings.Ports = reflect.MakeMap(t).Interface().(nat.PortMap)

	for i := 0; i < len(exportedPorts); i++ {
		c.Config.NetworkSettings.Ports[nat.Port(fmt.Sprintf("%d", i))] = []nat.PortBinding{
			nat.PortBinding{
				HostPort: fmt.Sprintf("%d", exportedPorts[i]),
			},
		}
	}

	ep := &docker_network.EndpointSettings{new(api_types.EndpointSettings), false}
	ep.NetworkID = overlayId
	ep.IPAddress = overlayIp
	c.Config.NetworkSettings.Networks[overlayId] = ep

	ep1 := &docker_network.EndpointSettings{new(api_types.EndpointSettings), false}
	ep1.NetworkID = "not used"
	ep1.IPAddress = "128.128.128.128" // a special test IP
	c.Config.NetworkSettings.Networks["bridge"] = ep1

	c.Config.ImageID = docker_image.ID(image)
	c.StaticPortMin = 0 //staticPortMin
	c.StaticPortMax = 0 //staticPortMax
	c.VmIps = []instanceIp{
		instanceIp{
			ns: DEFAULT_NS,
			ip: publicip,
		},
		instanceIp{
			ns: localns,
			ip: localip,
		},
	}
	c.LocalNs = localns
	c.Ips = []string{overlayIp, "128.128.128.128"}
	c.Id = id

	return c
}

/// create upon a non-created container
func newFreshReconcileCache(t *testing.T) *reconcileCache {
	c := newStubContainer("empty", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func newUpToDateReconcileCache(t *testing.T) *reconcileCache {
	c := newStubContainer("regular", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func newIpOnlyCache(t *testing.T) *reconcileCache {
	c := newStubContainer("iponly", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func newFactOnlyCache(t *testing.T) *reconcileCache {
	c := newStubContainer("stmtonly", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func newLinkOnlyCache(t *testing.T) *reconcileCache {
	c := newStubContainer("linkonly", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func newPortOnlyCache(t *testing.T) *reconcileCache {
	c := newStubContainer("portonly", "image-1", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	return newReconcileCache(metadata.NewStubApi(t), c)
}

func cacheTestDefault(cache *reconcileCache, t *testing.T) {
	if err := cache.Refresh(); err != nil {
		t.Logf("cant fetch server state, may be new container: %v\n", err)
	}
	if err := cache.Create(); err != nil {
		t.Fatalf("error in reconciling the state: %v\n", err)
	}

	state := cache.State()
	assert.NotNil(t, state, "state is not nil")
	serverState, err := cache.api.ShowPrincipal(cache.c.Id)
	if err != nil {
		t.Fatalf("error in showing principal: %v\n", err)
	}
	//t.Logf("server state: %v\nlocal state: %v\n", serverState, state)
	//assert.Equal(t, *state, *serverState, "compare state")
	t.Logf("cache state: %v\n\n, server state:%v\n\n", state, serverState)
	AssertPrincipalEqual(t, state, serverState)

}

func cacheTestNew(t *testing.T) {
	c := newStubContainer("newctn", "image-test", "192.168.0.1",
		"172.16.0.1", "localns", "10.0.0.1", "overlay",
		1000, 2000, 7077, 8088)
	cache := newReconcileCache(metadata.NewStubApi(t), c)
	cacheTestDefault(cache, t)

	if err := cache.Remove(); err != nil {
		t.Logf("fail to remove server state: %v\n", err)
	}

	err := cache.Refresh()
	assert.NotNil(t, err, "should not be able to refresh anymore")
}

func AssertPortAliasIn(t *testing.T, target []PortAlias, ns, ip, protocol string, min, max int) {
	assert.True(t, PortAliasIn(target, ns, ip, protocol, min, max), "slice found")
}

func PortAliasNotIn(t *testing.T, target []PortAlias, ns, ip, protocol string, min, max int) {
	found := false
	for _, p := range target {
		if p.nsName == ns && p.ip == ip && p.protocol == protocol &&
			p.min == min && p.max == max {
			found = true
			break
		}
	}
	assert.False(t, found, "slice found")
}

func TestPortAliasDiff(t *testing.T) {

	cports := []PortAlias{
		PortAlias{
			nsName:   "ns-1",
			ip:       "192.168.1.1",
			protocol: "udp",
			min:      1000,
			max:      2000,
		},
		PortAlias{
			nsName:   "ns-1",
			ip:       "192.168.1.1",
			protocol: "tcp",
			min:      1000,
			max:      2000,
		},
		PortAlias{
			nsName:   "ns-1",
			ip:       "192.168.1.1",
			protocol: "tcp",
			min:      4000,
			max:      4000,
		},
		PortAlias{
			nsName:   "ns-2",
			ip:       "192.168.2.1",
			protocol: "udp",
			min:      1000,
			max:      2000,
		},
		PortAlias{
			nsName:   "ns-2",
			ip:       "192.168.2.1",
			protocol: "tcp",
			min:      1000,
			max:      2000,
		},
		PortAlias{
			nsName:   "default",
			ip:       "192.168.0.1",
			protocol: "tcp",
			min:      1000,
			max:      2000,
		},
		PortAlias{
			nsName:   "default",
			ip:       "192.168.0.1",
			protocol: "udp",
			min:      1000,
			max:      2000,
		},
	}

	p := metadata.Principal{}

	ps1, ps2, ps3 := PortsAliasDiff(cports, &p)
	assert.Equal(t, cports, ps1, "compare empty principal, client only ports")
	assert.Len(t, ps2, 0, "compare empty principal, server only ports")
	assert.Len(t, ps3, 0, "compare empty principal, mutual ports")

	p.AddPortAlias("ns-1", "192.168.1.1", "tcp", 1000, 2000)
	p.AddPortAlias("ns-1", "192.168.1.1", "udp", 1000, 2000)
	p.AddPortAlias("ns-1", "192.168.1.1", "tcp", 2500, 3000)
	ps1, ps2, ps3 = PortsAliasDiff(cports, &p)
	assert.Len(t, ps3, 2, "adding ns-1 principal, mutual ports")
	AssertPortAliasIn(t, ps3, "ns-1", "192.168.1.1", "tcp", 1000, 2000)
	AssertPortAliasIn(t, ps3, "ns-1", "192.168.1.1", "udp", 1000, 2000)
	assert.Len(t, ps2, 1, "adding ns-1 principal, server ports")
	AssertPortAliasIn(t, ps2, "ns-1", "192.168.1.1", "tcp", 2500, 3000)

	p.AddPortAlias("ns-2", "192.168.2.1", "tcp", 1000, 2000)
	p.AddPortAlias("ns-2", "192.168.2.1", "udp", 1000, 2000)
	p.AddPortAlias("ns-2", "192.168.2.2", "udp", 1000, 1500)
	ps1, ps2, ps3 = PortsAliasDiff(cports, &p)
	AssertPortAliasIn(t, ps2, "ns-2", "192.168.2.2", "udp", 1000, 1500)
	AssertPortAliasIn(t, ps3, "ns-2", "192.168.2.1", "tcp", 1000, 2000)
	AssertPortAliasIn(t, ps3, "ns-2", "192.168.2.1", "udp", 1000, 2000)

	p.AddPortAlias("default", "192.168.0.1", "tcp", 1000, 2000)
	ps1, ps2, ps3 = PortsAliasDiff(cports, &p)
	AssertPortAliasIn(t, ps3, "default", "192.168.0.1", "tcp", 1000, 2000)
	p.AddPortAlias("ns-4", "192.168.3.1", "tcp", 1000, 2000)
	ps1, ps2, ps3 = PortsAliasDiff(cports, &p)
	AssertPortAliasIn(t, ps2, "ns-4", "192.168.3.1", "tcp", 1000, 2000)
}

type IpAliasByNsName []metadata.IpAlias

func (ips IpAliasByNsName) Len() int      { return len(ips) }
func (ips IpAliasByNsName) Swap(i, j int) { ips[i], ips[j] = ips[j], ips[i] }
func (ips IpAliasByNsName) Less(i, j int) bool {
	if ips[i].NsName == ips[j].NsName {
		return ips[i].Ip < ips[j].Ip
	} else {
		return ips[i].NsName < ips[j].NsName
	}
}

func AssertLinksIn(t *testing.T, p *metadata.Principal, link string) {
	for _, l := range p.Links {
		if l == link {
			return
		}
	}
	t.Fatalf("link (%s) missing\n", link)
}

func AssertStatementIn(t *testing.T, p *metadata.Principal, statement string) {
	for _, stmt := range p.Statements {
		if stmt.Fact == statement {
			return
		}
	}
	t.Fatalf("statement (%s) missing\n", statement)
}

func AssertIpAliasIn(t *testing.T, p *metadata.Principal, ns, ip string) {
	for _, alias := range p.Aliases.Ips {
		if alias.NsName == ns && alias.Ip == ip {
			return
		}
	}
	t.Fatalf("ip alias (%s,%s) missing\n", ns, ip)
}

/// assert dst's ports are all included in src
func AssertPortAliasContained(t *testing.T, src, dst *metadata.Principal) {
	for _, palias := range dst.Aliases.Ports {
		for _, ports := range palias.Ports.Tcp {
			_, j := src.FindPortAlias(palias.NsName, palias.Ip, "tcp", ports[0], ports[1])
			assert.NotEqual(t, j, -1, "tcp port alias check")
		}
		for _, ports := range palias.Ports.Udp {
			_, j := src.FindPortAlias(palias.NsName, palias.Ip, "udp", ports[0], ports[1])
			assert.NotEqual(t, j, -1, "udp port alias check")
		}
	}

}

func AssertPrincipalEqual(t *testing.T, p1 *metadata.Principal, p2 *metadata.Principal) {
	for _, ipalias := range p1.Aliases.Ips {
		AssertIpAliasIn(t, p2, ipalias.NsName, ipalias.Ip)
	}
	for _, ipalias := range p2.Aliases.Ips {
		AssertIpAliasIn(t, p1, ipalias.NsName, ipalias.Ip)
	}
	for _, link := range p1.Links {
		AssertLinksIn(t, p2, link)
	}
	for _, link := range p2.Links {
		AssertLinksIn(t, p1, link)
	}
	for _, stmt := range p2.Statements {
		AssertStatementIn(t, p1, stmt.Fact)
	}
	for _, stmt := range p1.Statements {
		AssertStatementIn(t, p2, stmt.Fact)
	}
	AssertPortAliasContained(t, p1, p2)
	AssertPortAliasContained(t, p2, p1)
}

func TestNewContainer(t *testing.T) {
	cache := newFreshReconcileCache(t)
	log.Debugf("test fresh\n")
	cacheTestDefault(cache, t)

	cache = newUpToDateReconcileCache(t)
	log.Debugf("test update to date\n")
	cacheTestDefault(cache, t)

	cache = newFactOnlyCache(t)
	log.Debugf("test fact only\n")
	cacheTestDefault(cache, t)
	cache = newLinkOnlyCache(t)
	log.Debugf("test link only\n")
	cacheTestDefault(cache, t)
	cache = newIpOnlyCache(t)
	log.Debugf("test ip only\n")
	cacheTestDefault(cache, t)
	cache = newPortOnlyCache(t)
	log.Debugf("test port only\n")
	cacheTestDefault(cache, t)

	log.Debugf("test new\n")
	cacheTestNew(t)
}
