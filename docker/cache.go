package docker

import (
	"fmt"
	"log"
	"net"

	metadata "github.com/jerryz920/tapcon-monitor/statement"
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
		log.Printf("error in show principal: %s\n", err)
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
				log.Printf("statement %s existed\n", fclient)
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
			log.Printf("image existed\n")
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
			/// not an interested IP
			continue
		}
		alias := metadata.IpAlias{NsName: nsName, Ip: cip}
		if !found {
			err := r.api.CreateIPAlias(cid, nsName, net.ParseIP(cip))
			if err != nil {
				/// dont update this in server cache then
				log.Printf("fail to create IP alias %s, %s\n", nsName, cip)
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
				log.Printf("fail to delete IP alias %s, %s\n", sip.NsName, sip.Ip)
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
	cid := tapconContainerId(r.c)
	if r.serverState == nil {
		err := r.api.CreatePrincipal(cid)
		if err != nil {
			return err
		}
		r.serverState = metadata.NewPrincipal()
	}

	if err := r.ReconcileFactStatement(); err != nil {
		log.Printf("error in posting facts: %v\n", err)
	}

	if err := r.ReconcileImageLink(); err != nil {
		log.Printf("error in linking image: %v\n", err)
	}

	if err := r.ReconcileIpAlias(); err != nil {
		log.Printf("error in reconcile IP aliases: %v\n", err)
	}

	if err := r.ReconcilePortAlias(); err != nil {
		log.Printf("error in reconciling Port aliases: %v\n", err)
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
