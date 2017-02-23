package statement

import (
	"fmt"
	"log"
)

type Statement string

const (
	Base64Ratio float64 = 1.5
)

// Estimating the buffer size for converting n statement array to, may not use
// this actually, the encoding speed is slower than memory growth I think
func EstimateJsonBufferCap(nstmt []Statement) int {
	s := 0
	for _, i := range nstmt {
		added := float64(len(i)) * Base64Ratio
		s += int(added)
	}
	return s
}

type IpAlias struct {
	NsName string `json:"ns_name"`
	Ip     string `json:"ip"`
}

type ProtocolPorts struct {
	Tcp [][2]int `json:"tcp"`
	Udp [][2]int `json:"udp"`
}

type PortAlias struct {
	NsName string        `json:"ns_name"`
	Ip     string        `json:"ip"`
	Ports  ProtocolPorts `json:"ports"`
}

type PrincipalAliases struct {
	Ips   []IpAlias   `json:"ips"`
	Ports []PortAlias `json:"ports,omitempty"`
}

type EndorsedStatement struct {
	Endorser string `json:"endorser,omitempty"`
	Fact     string `json:"fact,omitempty"`
}

type Principal struct {
	Aliases    PrincipalAliases    `json:"alias,omitempty"`
	Links      []string            `json:"links,omitempty"`
	Statements []EndorsedStatement `json:"statements,omitempty"`
}

func NewPrincipal() *Principal {
	p := &Principal{}
	p.Aliases.Ips = make([]IpAlias, 0, 3)
	p.Aliases.Ports = make([]PortAlias, 0, 8)
	p.Links = make([]string, 0, 2)
	p.Statements = make([]EndorsedStatement, 0, 2)
	return p
}

func (p *Principal) FindPortAlias(ns, ip, protocol string,
	portMin, portMax int) (int, int) {

	if protocol == "tcp" {
		for i, alias := range p.Aliases.Ports {
			if alias.NsName != ns || alias.Ip != ip {
				continue
			}
			for j, tcpPorts := range alias.Ports.Tcp {
				if tcpPorts[0] == portMin && tcpPorts[1] == portMax {
					return i, j
				}
			}
			return i, -1
		}
	} else if protocol == "udp" {
		for i, alias := range p.Aliases.Ports {
			if alias.NsName != ns || alias.Ip != ip {
				continue
			}
			for j, udpPorts := range alias.Ports.Udp {
				if udpPorts[0] == portMin && udpPorts[1] == portMax {
					return i, j
				}
			}
			return i, -1
		}
	} else {
		log.Printf("unsupported protocol %s\n", protocol)
	}
	return -1, -1

}

func NewPortAlias(ns, ip, protocol string, portMin, portMax int) PortAlias {
	if protocol == "tcp" {
		return PortAlias{
			NsName: ns,
			Ip:     ip,
			Ports: ProtocolPorts{
				Tcp: [][2]int{[2]int{portMin, portMax}},
				Udp: [][2]int{},
			},
		}
	} else {
		return PortAlias{
			NsName: ns,
			Ip:     ip,
			Ports: ProtocolPorts{
				Udp: [][2]int{[2]int{portMin, portMax}},
				Tcp: [][2]int{},
			},
		}
	}
}

func (p *Principal) AddPortAlias(ns, ip, protocol string,
	portMin, portMax int) error {
	i, j := p.FindPortAlias(ns, ip, protocol, portMin, portMax)
	if j != -1 {
		return fmt.Errorf("port alias %s %s %s %d %d existed", ns, ip, protocol,
			portMin, portMax)
	}
	if i != -1 {
		if protocol == "tcp" {
			p.Aliases.Ports[i].Ports.Tcp = append(p.Aliases.Ports[i].Ports.Tcp,
				[2]int{portMin, portMax})
		} else if protocol == "udp" {
			p.Aliases.Ports[i].Ports.Udp = append(p.Aliases.Ports[i].Ports.Udp,
				[2]int{portMin, portMax})
		}
	} else {
		p.Aliases.Ports = append(p.Aliases.Ports, NewPortAlias(ns, ip, protocol,
			portMin, portMax))
	}

	return nil
}

func (p *Principal) DelPortAlias(ns, ip, protocol string,
	portMin, portMax int) error {
	i, j := p.FindPortAlias(ns, ip, protocol, portMin, portMax)
	if j == -1 {
		return fmt.Errorf("port alias %s %s %s %d %d not found", ns, ip, protocol,
			portMin, portMax)
	}

	if protocol == "tcp" {
		ptr := &p.Aliases.Ports[i].Ports.Tcp
		*ptr = append((*ptr)[0:j], (*ptr)[j+1:]...)
	}
	return nil
}
