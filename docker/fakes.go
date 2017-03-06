package docker

import (
	"fmt"
	"os/exec"
)

/// just for testing
type fakeSandbox struct{}

func (s *fakeSandbox) ContainerChainName(id string) string {
	if len(id) <= 10 {
		return fmt.Sprintf("ctn-%s", id)
	}
	return fmt.Sprintf("ctn-%s", id[0:10])
}

func (s *fakeSandbox) SetupContainerChain(id string) error {
	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-N", chainName)
	fmt.Printf("cmd %v\n", cmd.Args)
	cmd = exec.Command("iptables", "-t", "nat", "-I", "POSTROUTING", "-j", chainName)
	fmt.Printf("cmd %v\n", cmd.Args)
	return nil
}

func (s *fakeSandbox) RemoveContainerChain(id string) error {
	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-X", chainName)
	fmt.Printf("cmd %v\n", cmd.Args)
	return nil
}

func (s *fakeSandbox) SetupStaticPortMapping(id string, containerIp string,
	portMin int, portMax int) error {

	chainName := s.ContainerChainName(id)
	for _, proto := range [2]string{"tcp", "udp"} {
		cmd := exec.Command("iptables", "-t", "nat", "-A", chainName,
			"-p", proto, "-d", containerIp,
			"-j", "MASQUERADE", "--to-ports", fmt.Sprintf("%d-%d", portMin, portMax))
		fmt.Printf("cmd %v\n", cmd.Args)
	}
	return nil
}

func (s *fakeSandbox) ClearStaticPortMapping(id string) error {
	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-F", chainName)
	fmt.Printf("cmd %v\n", cmd.Args)
	return nil
}
