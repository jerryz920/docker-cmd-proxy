package statement

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

/// Just for debugging purpose
type EmptyStubApi struct{}

func (s *EmptyStubApi) UploadVmImage(name, location, gitrepo, rev, format string,
	encoded bool) error {
	fmt.Printf("UploadVmImage: %s %s %s %s %s %v\n", name, location,
		gitrepo, rev, format, encoded)
	return nil
}

func (s *EmptyStubApi) MyId() (string, error) {
	fmt.Printf("myid\n")
	return "Id", nil
}

func (s *EmptyStubApi) MyNs() (string, error) {
	fmt.Printf("myns\n")
	return "Ns", nil
}

func (s *EmptyStubApi) CreatePrincipal(name string) error {
	fmt.Printf("createPrincipal %s\n", name)
	return nil
}

func (s *EmptyStubApi) ListPrincipals() (map[string]Principal, error) {
	fmt.Printf("listPrincipal\n")
	return make(map[string]Principal), nil
}

func (s *EmptyStubApi) ShowPrincipal(target string) (*Principal, error) {
	fmt.Printf("ShowPrincipal\n")
	return nil, nil
}

func (s *EmptyStubApi) DeletePrincipal(name string) error {
	fmt.Printf("DeletePrincipal %s\n", name)
	return nil
}

func (s *EmptyStubApi) CreateNs(name string) error {
	fmt.Printf("CreateNs %s\n", name)
	return nil
}

func (s *EmptyStubApi) JoinNs(name string) error {
	fmt.Printf("JoinNs %s\n", name)
	return nil
}

func (s *EmptyStubApi) LeaveNs(name string) error {
	fmt.Printf("LeaveNs %s\n", name)
	return nil
}

func (s *EmptyStubApi) DeleteNs(name string) error {
	fmt.Printf("DeleteNs %s\n", name)
	return nil
}

func (s *EmptyStubApi) CreateIPAlias(name string, ns string, ip net.IP) error {
	fmt.Printf("CreateIpAlias %s %s %v\n", name, ns, ip.String())
	return nil
}

func (s *EmptyStubApi) DeleteIPAlias(name string, ns string, ip net.IP) error {
	fmt.Printf("DeleteIpAlias %s %s %v\n", name, ns, ip.String())
	return nil
}

func (s *EmptyStubApi) CreatePortAlias(name string, ns string, ip net.IP, protocol string, portMin, portMax int) error {
	fmt.Printf("CreatePortAlias %s %s %v %s %d %d\n", name, ns, ip.String(), protocol, portMin, portMax)
	return nil
}

func (s *EmptyStubApi) DeletePortAlias(name string, ns string, ip net.IP, protocol string, portMin, portMax int) error {
	fmt.Printf("DeletePortAlias %s %s %v %s %d %d\n", name, ns, ip.String(), protocol, portMin, portMax)
	return nil
}

func (s *EmptyStubApi) PostProof(target string, statements []Statement) error {
	fmt.Printf("PostProof %s %v\n", target, statements)
	return nil
}

func (s *EmptyStubApi) LinkProof(target string, dependencies []string) error {
	fmt.Printf("LinkProof %s %v\n", target, dependencies)
	return nil
}

func (s *EmptyStubApi) PostProofForChild(cid string, statements []Statement) error {
	fmt.Printf("PostProofForChild %s, %v\n", cid, statements)
	return nil
}

func (s *EmptyStubApi) LinkProofForChild(cid string, links []string) error {
	fmt.Printf("LinkProofForChild %s, %v\n", cid, links)
	return nil
}
func (s *EmptyStubApi) SelfCertify(statements []Statement) error {
	fmt.Printf("SelfCertify %v\n", statements)
	return nil
}

func (s *EmptyStubApi) MyLocalIp() (string, error) {
	fmt.Printf("MyLocalIp\n")
	return "192.168.0.1", nil
}

func (s *EmptyStubApi) MyPublicIp() (string, error) {
	fmt.Printf("MyPublicIp\n")
	return "166.111.68.162", nil
}

type CallArgs struct {
	args []interface{}
}

func NewCall(args ...interface{}) *CallArgs {
	///FIXME: we need actually deep copy here.
	return &CallArgs{args}
}

/// stub api that works as a fake
type StubApi struct {
	*EmptyStubApi
	principals map[string]*Principal
	t          *testing.T
	callStat   map[string][]*CallArgs
}

func (api *StubApi) called(id string, args ...interface{}) {
	if _, ok := api.callStat[id]; ok {
		api.callStat[id] = append(api.callStat[id], NewCall(args))
		return
	}
	api.callStat[id] = []*CallArgs{NewCall(args)}
}

func loadPrincipal(content string) Principal {

	buffer := strings.NewReader(content)
	decoder := json.NewDecoder(buffer)
	var p Principal
	if err := decoder.Decode(&p); err != nil {
		log.Fatalf("error decoding json file: %v\n %s\n", err, content)
	}
	return p
}

func emptyPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {
            "ips": [],
            "ports": []
        },
        "links": [],
        "statements": []
    }`)
}

func ipOnlyPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {
            "ips": [
                {
                    "ns_name": "overlay",
                    "ip": "10.0.0.1"
                }
            ],
            "ports": [ ]
        },
        "links": [],
        "statements": []
    }`)
}

func portOnlyPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {
            "ips": [ ],
            "ports": [
                {
                    "ns_name": "default",
                    "ip": "192.168.0.1",
                    "ports": {
                        "tcp": [
                        [1000,2000],
			[7077,7077],
			[8088,8088]
                        ],
                        "udp": [
			[1000,2000],
			[7077,7077],
			[8088,8088]
			]
                    }
                },
                {
                    "ns_name": "localns",
                    "ip": "172.16.0.1",
                    "ports": {
                        "tcp": [
                        [1000,2000],
			[7077,7077],
			[8088,8088]
                        ],
                        "udp": [
			[1000,2000],
			[7077,7077],
			[8088,8088]
			]
                    }
                }
            ]
        },
        "links": [],
        "statements": []
    }`)
}

func regularPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {
            "ips": [
                {
                    "ns_name": "overlay",
                    "ip": "10.0.0.1"
                }
            ],
            "ports": [
                {
                    "ns_name": "default",
                    "ip": "192.168.0.1",
                    "ports": {
                        "tcp": [
                        [1000,2000],
			[7077,7077],
			[8088,8088]
                        ],
                        "udp": [
			[1000,2000],
			[7077,7077],
			[8088,8088]
			]
                    }
                },
                {
                    "ns_name": "localns",
                    "ip": "172.16.0.1",
                    "ports": {
                        "tcp": [
                        [1000,2000],
			[7077,7077],
			[8088,8088]
                        ],
                        "udp": [
			[1000,2000],
			[7077,7077],
			[8088,8088]
			]
                    }
                }
            ]
        },
        "links": ["image-1"],
        "statements": [
	{
	  "endorser": "self",
	  "fact": "containerFact(\"regular\", \"image-1\")"
      }
        ]
    }`)
}

func statementOnlyPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {},
        "links": [],
        "statements": [
	{
	  "endorser": "self",
	  "fact": "containerFact(\"regular\", \"image-1\")"
      }
        ]
    }`)
}

func linkOnlyPrincipal() Principal {
	return loadPrincipal(`{
        "alias": {},
        "links": ["image-1"],
        "statements": []
    }`)
}

func (api *StubApi) Reset() {
	api.principals = make(map[string]*Principal)
	api.callStat = make(map[string][]*CallArgs)
}

func (api *StubApi) CreatePrincipal(id string) error {
	api.called("CreatePrincipal", id)
	if id == "empty" ||
		id == "iponly" ||
		id == "portonly" ||
		id == "stmtonly" ||
		id == "linkonly" ||
		id == "regular" {
		api.t.Errorf("should not create pre-defined test principal")
	}

	if _, ok := api.principals[id]; ok {
		return fmt.Errorf("Principal %v has existed\n", id)
	}

	p := emptyPrincipal()
	api.principals[id] = &p
	return nil
}

func (api *StubApi) ShowPrincipal(id string) (*Principal, error) {
	api.called("ShowPrincipal", id)
	if ptr, ok := api.principals[id]; ok {
		copy := *ptr
		return &copy, nil
	}

	var p Principal
	if id == "empty" {
		p = emptyPrincipal()
	} else if id == "iponly" {
		p = ipOnlyPrincipal()
	} else if id == "portonly" {
		p = portOnlyPrincipal()
	} else if id == "stmtonly" {
		p = statementOnlyPrincipal()
	} else if id == "linkonly" {
		p = linkOnlyPrincipal()
	} else if id == "regular" {
		p = regularPrincipal()
	} else {
		return nil, fmt.Errorf("principal not found")
	}
	api.principals[id] = &p
	copy := p
	assert.False(api.t, &copy == &p, "principal address\n\n")
	return &copy, nil
}
func (api *StubApi) DeletePrincipal(id string) error {
	if id == "empty" ||
		id == "iponly" ||
		id == "portonly" ||
		id == "stmtonly" ||
		id == "linkonly" ||
		id == "regular" {
		api.t.Errorf("should not create pre-defined test principal")
	}
	if _, ok := api.principals[id]; ok {
		delete(api.principals, id)
		return nil
	} else {
		return fmt.Errorf("can not delete: principal not found")
	}

}

func (api *StubApi) CopySlice(src interface{}) interface{} {

	sv := reflect.ValueOf(src)
	assert.True(api.t, sv.Kind() == reflect.Slice || sv.Kind() == reflect.Array,
		"src slice type check")
	// just to unify access of both array and slice
	dstType := reflect.SliceOf(reflect.TypeOf(src).Elem())
	dst := reflect.MakeSlice(dstType, sv.Len(), sv.Cap())
	n := reflect.Copy(dst, sv)
	assert.Equal(api.t, n, sv.Len(), "slice copied length check")
	return dst.Interface()
}

func (api *StubApi) PostProofForChild(id string, statements []Statement) error {
	dstarg := api.CopySlice(statements)
	api.called("PostProofForChild", id, dstarg)
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	for _, s := range statements {
		ptr.Statements = append(ptr.Statements, EndorsedStatement{
			Endorser: "",
			Fact:     string(s),
		})
	}
	return nil
}

func (api *StubApi) LinkProofForChild(id string, links []string) error {
	dstarg := api.CopySlice(links)
	api.called("LinkProofForChild", id, dstarg)
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	for _, l := range links {
		ptr.Links = append(ptr.Links, l)
	}
	return nil
}

func (api *StubApi) CreateIPAlias(id string, ns string, ip net.IP) error {

	api.called("CreateIpAlias", id, ns, ip.String())
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	for _, alias := range ptr.Aliases.Ips {
		if alias.NsName == ns && alias.Ip == ip.String() {
			return fmt.Errorf("ip alias %s %v existed for %s", ns, ip, id)
		}
	}
	ptr.Aliases.Ips = append(ptr.Aliases.Ips, IpAlias{ns, ip.String()})
	return nil
}

func (api *StubApi) DeleteIPAlias(id string, ns string, ip net.IP) error {
	api.called("DeleteIpAlias", id, ns, ip.String())
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	found := -1
	for i, alias := range ptr.Aliases.Ips {
		if alias.NsName == ns && alias.Ip == ip.String() {
			found = i
			break
		}
	}
	if found == -1 {
		return fmt.Errorf("can not find ip alias: %s %v for %s", ns, ip, id)
	}
	ptr.Aliases.Ips = append(ptr.Aliases.Ips[0:found], ptr.Aliases.Ips[found+1:]...)
	return nil
}

func (api *StubApi) CreatePortAlias(id, ns string, ip net.IP, protocol string,
	portMin, portMax int) error {

	api.called("CreatePortAlias", id, ns, ip.String(), protocol, portMin, portMax)
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	return ptr.AddPortAlias(ns, ip.String(), protocol, portMin, portMax)
}

func (api *StubApi) DeletePortAlias(id, ns string, ip net.IP, protocol string,
	portMin, portMax int) error {
	api.called("DeletePortAlias", id, ns, ip.String(), protocol, portMin, portMax)
	ptr, ok := api.principals[id]

	if !ok {
		return fmt.Errorf("can not find principal %s\n", id)
	}
	return ptr.DelPortAlias(ns, ip.String(), protocol, portMin, portMax)
}

/// not a test case
func NewStubApi(t *testing.T) MetadataAPI {
	return &StubApi{
		EmptyStubApi: &EmptyStubApi{},
		principals:   make(map[string]*Principal),
		t:            t,
		callStat:     make(map[string][]*CallArgs),
	}
}
