package docker

import (
	"fmt"

	docker "github.com/docker/docker/container"
	docker_network "github.com/docker/docker/daemon/network"
	docker_image "github.com/docker/docker/image"
	nat "github.com/docker/go-connections/nat"
)

func newStubContainer(id, image, publicip, localip, localns string,
	overlayIp, overlayId string,
	staticPortMin, staticPortMax int, exportedPorts ...int) *MemContainer {

	c := &MemContainer{
		Config: docker.Container{},
	}
	c.Config.NetworkSettings = &docker_network.Settings{
		Networks: make(map[string]*docker_network.EndpointSettings),
		Ports:    nat.PortMap,
	}
	for i := 0; i < len(exportedPorts); i++ {
		c.Config.NetworkSettings.Ports[fmt.Sprintf("%d", i)] = []nat.PortBindings{
			PortBindings{
				HostPort: exportedPorts[i],
			},
		}
	}
	c.Config.NetworkSettings.Networks[overlayId] = &docker_network.EndpointSettings{
		NetworkID: overlayId,
		IPAddress: overlayIp,
	}
	c.Config.ImageID = docker_image.ID(image)
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
	c.Ips = []string{overlayIp}
	c.Id = id

	return c
}

/// create upon a non-created container
func newFreshReconcileCache() *reconcileCache {

}

func newUpToDateReconcileCache() *reconcileCache {

}

func newIpUnsyncedCache() *reconcileCache {
}

func newFactUnsyncedCache() *reconcileCache {
}

func newLinkUnsyncedCache() *reconcileCache {
}

func newPortUnsyncedCache() *reconcileCache {
}
