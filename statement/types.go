package statement

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
	NsName string `json:"ns_name"`
	Ip     string `json:"ip"`
	Ports  ProtocolPorts
}

type PortAliases struct {
	Ips   []IpAlias   `json:"ips"`
	Ports []PortAlias `json:"ports,omitempty"`
}
