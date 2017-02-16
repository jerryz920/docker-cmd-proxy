package statement

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	example1 string = `{
    "ips": [
    {
        "ns_name": "default",
        "ip": "192.168.1.1"
    },
    {
        "ns_name": "test",
        "ip": "172.16.0.1"
    }
    ],

    "ports": [
    {
        "ns_name": "default",
        "ip": "172.16.0.2",
        "ports": {
            "tcp": [[1,2], [2,3], [3,4], [4,5]],
            "udp": []
        }
    },
    {
        "ns_name": "overlay",
        "ip": "10.10.1.1",
        "ports": {
            "tcp": [[1,2], [2,3], [3,4], [4,5]],
            "udp": []
        }
    }
    ]
} `
	example2 string = `{
    "ips": [
    {
        "ns_name": "default",
        "ip": "192.168.1.1"
    },
    {
        "ns_name": "test",
        "ip": "172.16.0.1"
    }
    ]
    }`
	example3 string = `{
    "ips": [],

    "ports": [
    {
        "ns_name": "default",
        "ip": "172.16.0.2",
        "ports": {
            "tcp": [[1,2], [2,3], [3,4], [4,5]],
            "udp": []
        }
    },
    {
        "ns_name": "overlay",
        "ip": "10.10.1.1",
        "ports": {
            "tcp": [[1,2], [2,3], [3,4], [4,5]],
            "udp": []
        }
    }]
    }`
)

func TestLoadPrincipal(t *testing.T) {
	var p PortAliases
	decoder := json.NewDecoder(strings.NewReader(example1))
	if err := decoder.Decode(&p); err != nil {
		t.Fatalf("error: %v\n", err)
	}
	assert.Equal(t, p.Ips[0].NsName, "default", "ns_name should be equal")
	assert.Equal(t, p.Ports[1].Ports.Tcp[0], [2]int{1, 2}, "tcp ports")

	var p1 PortAliases
	decoder = json.NewDecoder(strings.NewReader(example2))
	if err := decoder.Decode(&p1); err != nil {
		t.Fatalf("error: %v\n", err)
	}
	assert.Len(t, p1.Ports, 0, "skipped ports")

	var p2 PortAliases
	decoder = json.NewDecoder(strings.NewReader(example3))
	if err := decoder.Decode(&p2); err != nil {
		t.Fatalf("error: %v\n", err)
	}
	assert.Len(t, p2.Ips, 0, "skipped ips")
}
