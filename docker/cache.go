package docker

import (
	"fmt"
	"math/rand"
	"net"

	log "github.com/Sirupsen/logrus"
	metadata "github.com/jerryz920/tapcon-monitor/statement"
)

var (
	Hotcloud17Workaround    = true
	Hotcloud17TapconPort    = 1000
	Hotcloud17ContainerPort = 2000
)

type ReconcileCache interface {
	Refresh() error
	Create() error
	Remove() error
	State() *metadata.Principal
	Valid() bool
}

type reconcileCache struct {
	api         metadata.MetadataAPI
	c           *MemContainer
	serverState *metadata.Principal
}

func (r *reconcileCache) Refresh() error {
	cid := tapconContainerId(r.c)
	p, err := r.api.ShowPrincipal(cid)
	if err != nil {
		if r.serverState != nil {
			log.Errorf("cache exists, but at server side: %s", err)
		} else {
			log.Debugf("unsynced principal in show: %s", err)
		}
		return err
	}
	r.serverState = p
	return nil
}

func (r *reconcileCache) ReconcileFactStatement() error {
	/// defensive
	if r.serverState == nil {
		return fmt.Errorf("must have valid server state to call this\n")
	}
	cid := tapconContainerId(r.c)

	/// check statements
	facts := r.c.ContainerFacts()
	toPost := make([]metadata.Statement, 0, len(facts))
	for _, fclient := range facts {
		found := false
		for _, fserver := range r.serverState.Statements {
			if string(fclient) == fserver.Fact {
				log.Debugf("statement %s existed", fclient)
				break
			}
		}
		if !found {
			toPost = append(toPost, fclient)
			break
		}
	}
	if len(toPost) > 0 {
		for _, f := range toPost {
			// We don't care about endorser here
			s := metadata.EndorsedStatement{Endorser: "", Fact: string(f)}
			r.serverState.Statements = append(r.serverState.Statements, s)
		}
		return r.api.PostProofForChild(cid, toPost)
	}
	return nil
}

func (r *reconcileCache) ReconcileImageLink() error {
	if r.serverState == nil {
		return fmt.Errorf("must have valid server state to call this\n")
	}
	cid := tapconContainerId(r.c)
	link := tapconContainerImageId(r.c)
	for _, slink := range r.serverState.Links {
		if slink == link {
			log.Debugf("image %s existed", slink)
			return nil
		}
	}
	r.serverState.Links = append(r.serverState.Links, link)
	return r.api.LinkProofForChild(cid, []string{link})
}

func (r *reconcileCache) ReconcileIpAlias() error {
	if r.serverState == nil {
		return fmt.Errorf("must have valid server state to call this\n")
	}
	cid := tapconContainerId(r.c)

	latestIpAliases := make([]metadata.IpAlias, 0, len(r.c.Ips))
	for _, cip := range r.c.Ips {
		found := false
		for _, sip := range r.serverState.Aliases.Ips {
			if cip == sip.Ip {
				found = true
				break
			}
		}
		nsName, err := r.c.GetNsName(cip)
		if err != nil {
			// this is a workaround: the network address allocated is
			// tenant network address, so it's using the instance local NS
			// name

			continue
		}
		alias := metadata.IpAlias{NsName: nsName, Ip: cip}
		if !found {
			err := r.api.CreateIPAlias(cid, nsName, net.ParseIP(cip))
			if err != nil {
				/// dont update this in server cache then
				log.Errorf("fail to create IP alias %s, %s", nsName, cip)
				continue
			}
		}
		latestIpAliases = append(latestIpAliases, alias)
	}

	for _, sip := range r.serverState.Aliases.Ips {
		found := false
		for _, cip := range r.c.Ips {
			if cip == sip.Ip {
				found = true
				break
			}
		}
		if !found {
			err := r.api.DeleteIPAlias(cid, sip.NsName, net.ParseIP(sip.Ip))
			if err != nil {
				/// dont update this in server cache then
				log.Errorf("fail to delete IP alias %s, %s", sip.NsName, sip.Ip)
				/// still include this in server cache, as there is error deleting
				latestIpAliases = append(latestIpAliases, sip)
				continue
			}
		}
	}
	r.serverState.Aliases.Ips = latestIpAliases
	return nil
}

func PortAliasIn(target []PortAlias, ns, ip, protocol string, min, max int) bool {
	for _, p := range target {
		if p.nsName == ns && p.ip == ip && p.protocol == protocol &&
			p.min == min && p.max == max {
			return true
		}
	}
	return false
}

func PortsAliasDiff(cports []PortAlias, p *metadata.Principal) (
	[]PortAlias, []PortAlias, []PortAlias) {

	clientOnly := make([]PortAlias, 0, len(cports))
	/// They won't differ much so just use len(cports)
	serverOnly := make([]PortAlias, 0, len(cports))
	mutualPorts := make([]PortAlias, 0, len(cports))

	for _, cport := range cports {
		_, j := p.FindPortAlias(cport.nsName, cport.ip, cport.protocol,
			cport.min, cport.max)
		if j != -1 {
			mutualPorts = append(mutualPorts, cport)
		} else {
			clientOnly = append(clientOnly, cport)
		}
	}

	for _, sports := range p.Aliases.Ports {
		for _, tcpPort := range sports.Ports.Tcp {
			if !PortAliasIn(cports, sports.NsName, sports.Ip, "tcp", tcpPort[0],
				tcpPort[1]) {
				serverOnly = append(serverOnly, PortAlias{
					min:      tcpPort[0],
					max:      tcpPort[1],
					protocol: "tcp",
					nsName:   sports.NsName,
					ip:       sports.Ip,
				})
			}
		}
		for _, udpPort := range sports.Ports.Udp {
			if !PortAliasIn(cports, sports.NsName, sports.Ip, "udp", udpPort[0],
				udpPort[1]) {
				serverOnly = append(serverOnly, PortAlias{
					min:      udpPort[0],
					max:      udpPort[1],
					protocol: "udp",
					nsName:   sports.NsName,
					ip:       sports.Ip,
				})
			}
		}
	}
	return clientOnly, serverOnly, mutualPorts

}

func (r *reconcileCache) ReconcilePortAlias() error {
	if r.serverState == nil {
		return fmt.Errorf("must have valid server state to call this\n")
	}
	cid := tapconContainerId(r.c)
	// PortAlias in this package is just a simple representation of properties
	// PortAlias from metadata package is the actual form of alias on the metadata
	// server
	ports := r.c.ContainerPorts()
	clientOnlyPorts, serverOnlyPorts, mutualPorts := PortsAliasDiff(
		ports, r.serverState)
	log.Debugf("----reconciling server port state----")
	log.Debugf("client ports: %v", clientOnlyPorts)
	log.Debugf("server ports: %v", serverOnlyPorts)
	log.Debugf("mutual ports: %v", mutualPorts)
	log.Debugf("-------------------------------------")

	for _, port := range clientOnlyPorts {
		ip := net.ParseIP(port.ip)
		if ip == nil {
			continue
		}
		if err := r.api.CreatePortAlias(cid, port.nsName, ip, port.protocol,
			port.min, port.max); err != nil {
			continue
		}
		mutualPorts = append(mutualPorts, port)
	}

	for _, port := range serverOnlyPorts {
		/// remove server ports so they are actually consistent with local state
		ip := net.ParseIP(port.ip)
		if ip == nil {
			mutualPorts = append(mutualPorts, port)
			continue
		}
		if err := r.api.DeletePortAlias(cid, port.nsName, ip, port.protocol,
			port.min, port.max); err != nil {
			// failure in deleting the alias, so still keep them in cache state.
			mutualPorts = append(mutualPorts, port)
			continue
		}
	}
	r.serverState.Aliases.Ports = make([]metadata.PortAlias, 0, len(mutualPorts))

	for _, port := range mutualPorts {
		r.serverState.AddPortAlias(port.nsName, port.ip, port.protocol,
			port.min, port.max)
	}

	return nil
}

/// Create principal if necessary
func (r *reconcileCache) Create() error {
	if Hotcloud17Workaround {
		log.Info("workaround")
		if !r.c.Posted && r.c.RepoStr != "" {
			r.c.Posted = true
			for _, vmip := range r.c.VmIps {
				if vmip.ns == DEFAULT_NS {
					last := rand.Intn(100)
					first := rand.Intn(30)
					ip := fmt.Sprintf("192.1.%d.%d", first+20, last+15)
					log.Infof("vmip %s, ip %s, repo %s", vmip.ip, ip, r.c.RepoStr)
					metadata.Hotcloud2017WorkaroundPostPrincipal(
						fmt.Sprintf("%s:%d", vmip.ip, Hotcloud17TapconPort),
						fmt.Sprintf("%s:%d", ip, Hotcloud17ContainerPort),
						tapconContainerId(r.c),
						r.c.RepoStr,
						r.c.Config.ImageID.String(),
					)
				}

			}
		}
		return nil
	}

	cid := tapconContainerId(r.c)
	if r.serverState == nil {
		err := r.api.CreatePrincipal(cid)
		if err != nil {
			return err
		}
		r.serverState = metadata.NewPrincipal()
	}

	/// just a workaround

	if err := r.ReconcileFactStatement(); err != nil {
		log.Errorf("error in posting facts: %v", err)
	}

	if err := r.ReconcileImageLink(); err != nil {
		log.Errorf("error in linking image: %v", err)
	}

	if err := r.ReconcileIpAlias(); err != nil {
		log.Errorf("error in reconcile IP aliases: %v", err)
	}

	if err := r.ReconcilePortAlias(); err != nil {
		log.Errorf("error in reconciling Port aliases: %v", err)
	}
	return nil
}

func (r *reconcileCache) Remove() error {
	cid := tapconContainerId(r.c)
	if r.serverState != nil {
		err := r.api.DeletePrincipal(cid)
		if err != nil {
			return err
		}
		r.serverState = nil
	}
	return nil
}

func (r *reconcileCache) State() *metadata.Principal {
	return r.serverState
}

func (r *reconcileCache) Valid() bool {
	/// internally we reuse the server state for valid indicator. But in fact
	// we should have different way to mark so
	return r.serverState != nil
}

func NewReconcileCache(api metadata.MetadataAPI, c *MemContainer) ReconcileCache {
	return &reconcileCache{
		api:         api,
		c:           c,
		serverState: nil,
	}
}

// just for test
func newReconcileCache(api metadata.MetadataAPI, c *MemContainer) *reconcileCache {
	return &reconcileCache{
		api:         api,
		c:           c,
		serverState: nil,
	}
}
