package docker

import (
	"net"
	"os/exec"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type NetworkEvent struct {
	Id  string
	Err error
}

type NetworkDelayFunc func(NetworkEvent) error

func getOverlayNetworks() []string {
	cmd := exec.Command("docker", "network", "ls", "--no-trunc", "-q", "-f", "driver=overlay")
	out, err := cmd.Output()
	if err != nil {
		log.Errorf("error in capturing network output: %v", err)
		return []string{}
	}
	trimmed := strings.Trim(string(out), "\n")
	return strings.Split(trimmed, "\n")
}

func (m *Monitor) NetworkChanges() ([]string, []string) {

	// check the delayed queue, this is actually pretty slow work so
	// we may do something to it
	m.NetworkWorkerLock.Lock()
	oldNetworks := m.Networks
	newNetworks := getOverlayNetworks()
	toDelete := []string{}
	toAdd := []string{}

	for _, o := range oldNetworks {
		found := false
		for _, n := range newNetworks {
			if o == n {
				found = true
				break
			}
		}
		if !found {
			toDelete = append(toDelete, o)
		}
	}

	for _, n := range newNetworks {
		found := false
		for _, o := range oldNetworks {
			if o == n {
				found = true
				break
			}
		}
		if !found {
			toAdd = append(toAdd, n)
		}
	}
	m.Networks = newNetworks
	m.NetworkWorkerLock.Unlock()
	return toAdd, toDelete
}

func (m *Monitor) setupInstanceIpInfo() {
	pubIp, err := m.MetadataApi.MyPublicIp()
	if err != nil {
		log.Fatalf("can not obtain public IP info: %s", err)
	}
	m.publicIp = net.ParseIP(pubIp)
	if m.publicIp == nil {
		log.Fatalf("invalid IP: %s", pubIp)
	}
	localIp, err := m.MetadataApi.MyLocalIp()
	if err != nil {
		log.Fatalf("can not obtain local IP info: %s", err)
	}
	m.localIp = net.ParseIP(localIp)
	if m.localIp == nil {
		log.Fatalf("invalid IP: %s", localIp)
	}
	localNs, err := m.MetadataApi.MyNs()
	if err != nil {
		log.Fatalf("can not obtain local Ns info: %s", err)
	}
	m.localNs = localNs
}

func (m *Monitor) setupPortMapping(cid string, pmin int, pmax int) error {
	// Let's make it simple: exposed ports only for the local and public
	// IPs of the instance, not the overlayed network
	if err := m.MetadataApi.CreatePortAlias(cid, m.localNs, m.localIp,
		"tcp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.CreatePortAlias(cid, m.localNs, m.localIp,
		"udp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.CreatePortAlias(cid, DEFAULT_NS, m.publicIp,
		"tcp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.CreatePortAlias(cid, DEFAULT_NS, m.publicIp,
		"udp", pmin, pmax); err != nil {
		return err
	}
	return nil
}

func (m *Monitor) tearPortMapping(cid string, pmin int, pmax int) error {
	if err := m.MetadataApi.DeletePortAlias(cid, m.localNs, m.localIp,
		"tcp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.DeletePortAlias(cid, m.localNs, m.localIp,
		"udp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.DeletePortAlias(cid, DEFAULT_NS, m.publicIp,
		"tcp", pmin, pmax); err != nil {
		return err
	}
	if err := m.MetadataApi.DeletePortAlias(cid, DEFAULT_NS, m.publicIp,
		"udp", pmin, pmax); err != nil {
		return err
	}
	return nil
}
