package statement

import (
	"fmt"
	"net"
)

/// Just for debugging purpose
type StubApi struct{}

func (s *StubApi) UploadVmImage(name, location, gitrepo, rev, format string,
	encoded bool) error {
	fmt.Printf("UploadVmImage: %s %s %s %s %s %v\n", name, location,
		gitrepo, rev, format, encoded)
	return nil
}

func (s *StubApi) MyId() (string, error) {
	fmt.Printf("myid\n")
	return "Id", nil
}

func (s *StubApi) MyNs() (string, error) {
	fmt.Printf("myns\n")
	return "Ns", nil
}

func (s *StubApi) CreatePrincipal(name string) error {
	fmt.Printf("createPrincipal %s\n", name)
	return nil
}

func (s *StubApi) ListPrincipals() ([]string, error) {
	fmt.Printf("listPrincipal\n")
	return []string{"p1", "p2"}, nil
}

func (s *StubApi) DeletePrincipal(name string) error {
	fmt.Printf("DeletePrincipal %s\n", name)
	return nil
}

func (s *StubApi) CreateNs(name string) error {
	fmt.Printf("CreateNs %s\n", name)
	return nil
}

func (s *StubApi) JoinNs(name string) error {
	fmt.Printf("JoinNs %s\n", name)
	return nil
}

func (s *StubApi) LeaveNs(name string) error {
	fmt.Printf("LeaveNs %s\n", name)
	return nil
}

func (s *StubApi) DeleteNs(name string) error {
	fmt.Printf("DeleteNs %s\n", name)
	return nil
}

func (s *StubApi) CreateIPAlias(name string, ns string, ip net.IP) error {
	fmt.Printf("CreateIpAlias %s %s %v\n", name, ns, ip.String())
	return nil
}

func (s *StubApi) DeleteIPAlias(name string, ns string, ip net.IP) error {
	fmt.Printf("DeleteIpAlias %s %s %v\n", name, ns, ip.String())
	return nil
}

func (s *StubApi) CreatePortAlias(name string, ns string, ip net.IP, protocol string, portMin, portMax int) error {
	fmt.Printf("CreatePortAlias %s %s %v %s %d %d\n", name, ns, ip.String(), protocol, portMin, portMax)
	return nil
}

func (s *StubApi) DeletePortAlias(name string, ns string, ip net.IP, protocol string, portMin, portMax int) error {
	fmt.Printf("DeletePortAlias %s %s %v %s %d %d\n", name, ns, ip.String(), protocol, portMin, portMax)
	return nil
}

func (s *StubApi) PostProof(target string, statements []Statement) error {
	fmt.Printf("PostProof %s %v\n", target, statements)
	return nil
}

func (s *StubApi) LinkProof(target string, dependencies []string) error {
	fmt.Printf("LinkProof %s %v\n", target, dependencies)
	return nil
}

func (s *StubApi) MyLocalIp() (string, error) {
	fmt.Printf("MyLocalIp\n")
	return "192.168.0.1", nil
}

func (s *StubApi) MyPublicIp() (string, error) {
	fmt.Printf("MyPublicIp\n")
	return "166.111.68.162", nil
}
