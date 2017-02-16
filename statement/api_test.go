package statement

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	principalList = "[\"p1\", \"p2\", \"p3\"]"
)

var (
	logwriter io.Writer
	api       MetadataAPI
)

func getApiName(urlname string) string {
	parts := strings.Split(urlname, "/")
	if len(parts) == 0 {
		return ""
	}
	return "/" + parts[len(parts)-1]
}

func HandleUploadVmImage(w http.ResponseWriter, r *http.Request) {

	// check condition
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "false: read body %v", err)
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(string(data))

	if string(decoded) != "test string" {
		fmt.Fprintf(w, "false: decoding %v", string(decoded))
		return
	}

	params := r.URL.Query()
	if params.Get("image_git") != "git://" {
		fmt.Fprintf(w, "false: git %s", params.Get("image_git"))
		return
	}

	if params.Get("image_git_rev") != "0x12345" {
		fmt.Fprintf(w, "false: git rev %s", params.Get("image_git_rev"))
		return
	}

	if params.Get("image_disk_format") != "bare" {
		fmt.Fprintf(w, "false: format %s", params.Get("image_disk_format"))
		return
	}

	if params.Get("image_name") != "testimage" {
		fmt.Fprintf(w, "false: image name %s", params.Get("image_name"))
		return
	}

	fmt.Fprintf(w, "true")
}

func HandleViewPrincipalName(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "myname")
}

func HandlePostProof(w http.ResponseWriter, r *http.Request) {
	// check condition
	params := r.URL.Query()
	decoded, err := base64.StdEncoding.DecodeString(params.Get("statements"))
	if err != nil {
		fmt.Fprintf(w, "false: %v", err)
		return
	}

	buffer := bytes.NewBuffer(decoded)
	jdecoder := json.NewDecoder(buffer)
	var stmts []string
	if err := jdecoder.Decode(&stmts); err != nil {
		fmt.Fprintf(w, "false: %v", err)
		return
	}

	if !reflect.DeepEqual(stmts, []string{"stmt1", "stmt2", "stmt3"}) {
		fmt.Fprintf(w, "false: %v", stmts)
		return
	}

	if params.Get("target") != "target" {
		fmt.Fprintf(w, "false")
		return
	}
	fmt.Fprintf(w, "true")
}

func HandleLinkProof(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	decoded, err := base64.StdEncoding.DecodeString(params.Get("dependencies"))
	if err != nil {
		fmt.Fprintf(w, "false: %v", err)
		return
	}

	buffer := bytes.NewBuffer(decoded)
	jdecoder := json.NewDecoder(buffer)
	var deps []string
	if err := jdecoder.Decode(&deps); err != nil {
		fmt.Fprintf(w, "false: %v", err)
		return
	}

	if !reflect.DeepEqual(deps, []string{"dep1", "dep2", "dep3"}) {
		fmt.Fprintf(w, "false: %v", deps)
		return
	}

	if params.Get("target") != "target" {
		fmt.Fprintf(w, "false")
		return
	}
	fmt.Fprintf(w, "true")
}

func HandleListPrincipals(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, principalList)
}

func HandleNsName(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "myns")
}

func HandleViewLocalIP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "192.168.0.1")
}

func HandleViewPublicIP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "128.104.105.162")
}

type Handler func(http.ResponseWriter, *http.Request)

func checkFieldsFunc(fields map[string]string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		for k, v := range fields {
			if params.Get(k) != v {
				fmt.Fprintf(w, "false: %v != %v", params.Get(k), v)
				return
			}
		}
		fmt.Fprintf(w, "true")
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	urlname := r.URL.Path

	apiname := getApiName(urlname)

	/// manually crafted dispatch table. We could use mux for sure but
	// it's just a simple test. No need to do that so far.

	dispatcher_table := map[string]Handler{
		kUploadVmImage:     HandleUploadVmImage,
		kViewPrincipalName: HandleViewPrincipalName,
		kViewNs:            HandleNsName,
		kViewLocalIP:       HandleViewLocalIP,
		kViewPublicIP:      HandleViewPublicIP,
		kPostProof:         HandlePostProof,
		kLinkProof:         HandleLinkProof,
		kListPrincipals:    HandleListPrincipals,
		kCreatePrincipal: checkFieldsFunc(
			map[string]string{"principal": "target"},
		),
		kDeletePrincipal: checkFieldsFunc(
			map[string]string{"principal": "target"},
		),
		kCreateNs: checkFieldsFunc(
			map[string]string{"ns_name": "ns1"},
		),
		kDeleteNs: checkFieldsFunc(
			map[string]string{"ns_name": "ns1"},
		),
		kJoinNs: checkFieldsFunc(
			map[string]string{"ns_name": "ns1"},
		),
		kLeaveNs: checkFieldsFunc(
			map[string]string{"ns_name": "ns1"},
		),
		kCreateIPAlias: checkFieldsFunc(
			map[string]string{
				"principal": "p1",
				"ns_name":   "ns1",
				"ip":        "192.168.0.1",
			},
		),
		kDeleteIPAlias: checkFieldsFunc(
			map[string]string{
				"principal": "p1",
				"ns_name":   "ns1",
				"ip":        "192.168.0.1",
			},
		),
		kCreatePortAlias: checkFieldsFunc(
			map[string]string{
				"principal": "p1",
				"ns_name":   "ns1",
				"ip":        "192.168.0.1",
				"protocol":  "tcp",
				"port_min":  "1000",
				"port_max":  "2000",
			},
		),
		kDeletePortAlias: checkFieldsFunc(
			map[string]string{
				"principal": "p1",
				"ns_name":   "ns1",
				"ip":        "192.168.0.1",
				"protocol":  "tcp",
				"port_min":  "1000",
				"port_max":  "2000",
			},
		),
	}
	fmt.Fprintf(logwriter, "apiname %s\n", apiname)
	disp, ok := dispatcher_table[apiname]
	if !ok {
		fmt.Fprintf(w, "false\n")
	} else {
		disp(w, r)
	}
}

func StartEchoServer() {
	// open a http server that only sends back true/false, string and string list only
	logwriter = os.Stdout
	http.HandleFunc("/", handler)
	go http.ListenAndServe("127.0.0.1:2017", nil)
}

func init() {
	StartEchoServer()
	api = NewOpenstackMetadataAPI("127.0.0.1:2017")
}

func TestUploadVmImage(t *testing.T) {
	// write a temp file with binary content
	tmpd := os.TempDir()
	f, err := ioutil.TempFile(tmpd, "vmbinaryimage")
	if err != nil {
		t.Fatalf("creating temp file: %v\n", err)
	}
	filename := f.Name()
	defer os.Remove(filename)

	data := "test string"
	f.WriteString(data)
	f.Close()

	err = api.UploadVmImage("testimage", filename, "git://", "0x12345", "bare", false)
	if err != nil {
		t.Fatalf("uploading image: %v\n", err)
	}

}

func TestUploadVmImageBase64(t *testing.T) {
	tmpd := os.TempDir()
	f, err := ioutil.TempFile(tmpd, "vmbinaryimage")
	if err != nil {
		t.Fatalf("creating temp file: %v\n", err)
	}
	filename := f.Name()
	defer os.Remove(filename)

	data := "test string"
	f.WriteString(base64.StdEncoding.EncodeToString([]byte(data)))
	f.Close()

	err = api.UploadVmImage("testimage", filename, "git://", "0x12345", "bare", true)
	if err != nil {
		t.Fatalf("uploading image: %v\n", err)
	}
}

func TestMyId(t *testing.T) {
	s, err := api.MyId()
	if err != nil {
		t.Fatalf("error view ID: %v", err)
	}
	assert.Equal(t, s, "myname", "return value should be equal")
}

func TestMyNs(t *testing.T) {
	s, err := api.MyNs()
	if err != nil {
		t.Fatalf("error view NS: %v", err)
	}
	assert.Equal(t, s, "myns", "return value should be equal")
}

func TestCreatePrincipal(t *testing.T) {
	err := api.CreatePrincipal("target")
	if err != nil {
		t.Fatalf("error creating principal: %v", err)
	}
}

func TestListPrincipals(t *testing.T) {
	s, err := api.ListPrincipals()
	if err != nil {
		t.Fatalf("error listing principal: %v", err)
	}
	assert.EqualValues(t, s, []string{"p1", "p2", "p3"}, "principal list should equal")
}

func TestDeletePrincipal(t *testing.T) {
	err := api.DeletePrincipal("target")
	if err != nil {
		t.Fatalf("error deleting principal: %v", err)
	}
}

func TestCreateNs(t *testing.T) {
	err := api.CreateNs("ns1")
	if err != nil {
		t.Fatalf("error creating ns: %v", err)
	}
}

func TestJoinNs(t *testing.T) {
	err := api.JoinNs("ns1")
	if err != nil {
		t.Fatalf("error joining ns: %v", err)
	}
}

func TestLeaveNs(t *testing.T) {
	err := api.LeaveNs("ns1")
	if err != nil {
		t.Fatalf("error leaving ns: %v", err)
	}
}

func TestDeleteNs(t *testing.T) {
	err := api.DeleteNs("ns1")
	if err != nil {
		t.Fatalf("error deleting ns: %v", err)
	}
}

func TestCreateIPAlias(t *testing.T) {

	err := api.CreateIPAlias("p1", "ns1", net.ParseIP("192.168.0.1"))
	if err != nil {
		t.Fatalf("error creating ip alias: %v", err)
	}
}

func TestDeleteIPAlias(t *testing.T) {
	err := api.DeleteIPAlias("p1", "ns1", net.ParseIP("192.168.0.1"))
	if err != nil {
		t.Fatalf("error deleting ip alias: %v", err)
	}
}

func TestCreatePortAlias(t *testing.T) {
	err := api.CreatePortAlias("p1", "ns1", net.ParseIP("192.168.0.1"), "tcp", 1000, 2000)
	if err != nil {
		t.Fatalf("error creating port alias: %v", err)
	}
}

func TestDeletePortAlias(t *testing.T) {
	err := api.DeletePortAlias("p1", "ns1", net.ParseIP("192.168.0.1"), "tcp", 1000, 2000)
	if err != nil {
		t.Fatalf("error deleting port alias: %v", err)
	}
}

func TestPostProof(t *testing.T) {
	err := api.PostProof("target", []Statement{"stmt1", "stmt2", "stmt3"})
	if err != nil {
		t.Fatalf("error %v", err)
	}
}

func TestLinkProof(t *testing.T) {
	err := api.LinkProof("target", []string{"dep1", "dep2", "dep3"})
	if err != nil {
		t.Fatalf("error %v", err)
	}
}

func TestGetLocalIP(t *testing.T) {
	s, err := api.MyLocalIp()
	if err != nil {
		t.Fatalf("error listing local ip: %v", err)
	}
	assert.EqualValues(t, s, "192.168.0.1", "local ip list")
}

func TestGetPublicIP(t *testing.T) {
	s, err := api.MyPublicIp()
	if err != nil {
		t.Fatalf("error listing public ip: %v", err)
	}
	assert.EqualValues(t, s, "128.104.105.162", "public ip list")
}
