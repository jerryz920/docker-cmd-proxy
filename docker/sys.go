package docker

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

/// metadata and port management for tapcon monitor

func (m *Monitor) ProvisionContainer(id string, c *MemContainer) error {
	if id == "" {
		id = c.Config.ID
	}
	return nil
}

/// called after the container reloads, so
func (m *Monitor) PrincipalKeepAlive() {
}

/*
Tapcon metadata management:

    by default drop all communication to metadata server, setup a small portion

    when a config is created/updated, and container state == Run, create principal,
        create ip alias for every overlay network
        create port alias for every Exported Port on default network
        create port alias for a statically assigned port ranges
        link the image (must have existed) proofs of its container image

        insert ACCEPT rule for metadata server
        insert SNAT rule for statically assigned ports

    when container state == Stop, delete principal
        remove related SNAT rule
        remove links? should have been done when principal is removed

    when a new image is added:
        post proof for it.

    potential race: frequent start/stop the container/attach it/detach it from
    network, may cause damage to metadata server. We refresh the principal
    condition on a configured period after the initial creation. Each MemContainer
    has a lock, will set its lastUpdate time for metadata operation. The metadata
    operation scan will skip it
*/

// call our ipshow tool to display network namespace IPs. Docker bind the ns name
// to different places so ip-route tools can not make use of that.
func parseIps(data string) []string {
	data = strings.Trim(data, "\n")
	splitted := strings.Split(data, "\n")
	/// each line is <eth> <ip> format
	result := make([]string, 0, len(splitted))
	for i, ipline := range splitted {
		info := strings.Split(ipline, " ")
		if len(info) < 2 {
			log.Printf("error reading in IPs: %s [%s]\n", splitted[i], ipline)
			return []string{}
		}
		result = append(result, info[1])
	}
	return result
}

func ListNsIps(ns string) []string {

	cmd := exec.Command("ipshow", ns)
	data, err := cmd.Output()
	if err != nil {
		log.Printf("error in listing Ns %s Ips: %s\n", ns, err.Error())
		return []string{}
	}
	return parseIps(string(data))
}

/// Each chain contains the mapping of static ports assigned to it

type Sandbox interface {
	ContainerChainName(id string) string
	SetupContainerChain(id string) error
	RemoveContainerChain(id string) error
	SetupStaticPortMapping(id string, containerIp string,
		portMin int, portMax int) error
	ClearStaticPortMapping(id string) error
}

type sandbox struct{}

func (s *sandbox) ContainerChainName(id string) string {
	return fmt.Sprintf("ctn-%s", id)
}

func (s *sandbox) SetupContainerChain(id string) error {

	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-N", chainName)

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error creating chain: %s", string(out))
		return err
	}

	cmd = exec.Command("iptables", "-t", "nat", "-I", "POSTROUTING", "-j", chainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error inserting chain: %s", string(out))
		// we ignore the error here...
		exec.Command("iptables", "-t", "nat", "-X", chainName).Run()
		return err
	}
	return nil
}

func (s *sandbox) RemoveContainerChain(id string) error {
	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING", "-j", chainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error clearing jumping to static mapping chain: %s", string(out))
		return err
	}

	cmd = exec.Command("iptables", "-t", "nat", "-F", chainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error clearing static mapping chain: %s", string(out))
		return err
	}

	cmd = exec.Command("iptables", "-t", "nat", "-X", chainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error deleting static mapping chain: %s", string(out))
		return err
	}
	return nil
}

func (s *sandbox) SetupStaticPortMapping(id string, containerIp string,
	portMin int, portMax int) error {

	chainName := s.ContainerChainName(id)
	for _, proto := range [2]string{"tcp", "udp"} {
		cmd := exec.Command("iptables", "-t", "nat", "-A", chainName,
			"-p", proto, "-d", containerIp,
			"-j", "MASQUERADE", "--to-ports", fmt.Sprintf("%d-%d", portMin, portMax))
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("error inserting static mapping rule: %s", string(out))
			// clear the chain
			exec.Command("iptables", "-t", "nat", "-F", chainName).Run()
			return err
		}
	}
	return nil
}

func (s *sandbox) ClearStaticPortMapping(id string) error {
	chainName := s.ContainerChainName(id)
	cmd := exec.Command("iptables", "-t", "nat", "-F", chainName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("error clearing static mapping chain: %s", string(out))
		return err
	}
	return nil
}
