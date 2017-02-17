package statement

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

type MetadataAPI interface {
	UploadVmImage(name, location, gitrepo, rev, format string, encoded bool) error
	MyId() (string, error)
	MyNs() (string, error)
	CreatePrincipal(name string) error
	ListPrincipals() ([]string, error)
	DeletePrincipal(name string) error
	CreateNs(name string) error
	JoinNs(name string) error
	LeaveNs(name string) error
	DeleteNs(name string) error
	CreateIPAlias(name string, ns string, ip net.IP) error
	DeleteIPAlias(name string, ns string, ip net.IP) error
	CreatePortAlias(name string, ns string, ip net.IP, protocol string,
		portMin, portMax int) error
	DeletePortAlias(name string, ns string, ip net.IP, protocol string,
		portMin, portMax int) error

	PostProof(target string, statements []Statement) error
	PostProofForChild(target string, statements []Statement) error
	LinkProof(target string, dependencies []string) error
	LinkProofForChild(target string, dependencies []string) error
	SelfCertify(statements []Statement) error

	/// traditional metadata api
	MyLocalIp() (string, error)
	MyPublicIp() (string, error)
}

const (
	MetadataHost = "169.254.169.254"
	APIPath      = "openstack/latest/container_api"
	AwsAPIPath   = "latest/meta-data"
	/// API endpoints
	kUploadVmImage     = "/upload_tapcon_image"
	kViewNs            = "/query_iaas_ns"
	kViewPrincipalName = "/view_principal_name"
	kPostProof         = "/post_proofs"
	kPostProofForChild = "/post_proofs_for_child"
	kLinkProof         = "/link_proofs"
	kLinkProofForChild = "/link_proofs_for_child"
	kSelfCertify       = "/self_certify"
	kCreatePrincipal   = "/create_principal"
	kDeletePrincipal   = "/delete_principal"
	kListPrincipals    = "/list_principals"
	kCreateNs          = "/create_ns"
	kDeleteNs          = "/delete_ns"
	kJoinNs            = "/join_ns"
	kLeaveNs           = "/leave_ns"
	kCreateIPAlias     = "/create_ip_alias"
	kDeleteIPAlias     = "/delete_ip_alias"
	kCreatePortAlias   = "/create_port_alias"
	kDeletePortAlias   = "/delete_port_alias"
	// traditional AWS api:
	kViewLocalIP  = "/local-ipv4"
	kViewPublicIP = "/public-ipv4"

	/// Query parameters
	qImageGitRepo    = "image_git"
	qImageGitRev     = "image_git_rev"
	qImageName       = "image_name"
	qImageDiskFormat = "image_disk_format"
	qPrincipalName   = "principal"
	qNsName          = "ns_name"
	qIpAlias         = "ip"
	qProtocol        = "protocol"
	qPortMin         = "port_min"
	qPortMax         = "port_max"
	qTarget          = "target"
	qDependencies    = "dependencies"
	qStatements      = "statements"

	// some ratio to be tuned for statement posting
)

// tapcon implementation of api
type Api struct {
	client     *http.Client
	serverAddr string
}

// For whatever result the server is returning 200 at the moment. Though
// a "false" in body is returned for failure, and "true" for success
func ok(resp *http.Response) error {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error in reading metadata server result: %v\n", err)
		return err
	}
	res := string(data)
	res = strings.ToLower(res)
	// make it debug
	if res == "true" {
		return nil
	}
	return fmt.Errorf("metadata server returns error string: %s\n", res)
}

func strResp(resp *http.Response) (string, error) {
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error in reading metadata server result: %v\n", err)
		return "", err
	}
	return string(data), nil
}

func jsonResp(resp *http.Response) ([]string, error) {
	decoder := json.NewDecoder(resp.Body)
	result := make([]string, 0)
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func NewOpenstackMetadataAPI(addr string) MetadataAPI {
	if addr == "" {
		addr = MetadataHost
	}
	api := &Api{serverAddr: addr}

	tr := &http.Transport{
		// if we need TLS in future
		//TLSClientConfig: &tls.Config{RootCAs: pool},
		DisableCompression: true,
	}
	api.client = &http.Client{Transport: tr}
	return api
}

type Base64FileReader struct {
	reader io.ReadCloser
	writer io.WriteCloser // may not use this
}

// for a non-encoded file reading, we implement this as a prototype,
// but actually if we are encoding large files we will be screwed
// due to the memory consumption.
// TODO: move such things to utilities and make a "streaming"
// encoder which buffers file io and adapts to http post
func NewBinaryFileReader(path string) (*Base64FileReader, error) {
	encoder := &Base64FileReader{}
	encoder.reader, encoder.writer = io.Pipe()
	go func() {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Printf("Error in reading file %s: %s", path, err)
			encoder.Close()
		}
		e := base64.StdEncoding
		dst := make([]byte, e.EncodedLen(len(data)))
		e.Encode(dst, data)
		encoder.writer.Write(dst)
		encoder.writer.Close()
	}()
	return encoder, nil
}

func NewBase64FileReader(path string) (*Base64FileReader, error) {
	reader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	encoder := &Base64FileReader{reader: reader, writer: nil}
	return encoder, nil
}

func (e *Base64FileReader) Close() {
	if err := e.reader.Close(); err != nil {
		log.Printf("error in closing uploading reader %s\n", err)
	}
	// we dont need to close writer as it would be closed by goroutine after writing
	// is done
}

func (e *Base64FileReader) Read(data []byte) (int, error) {
	return e.reader.Read(data)
}

func (api *Api) GetAPI(schema, apiName string) string {
	url := fmt.Sprintf("%s://%s/%s%s", schema, api.serverAddr, APIPath, apiName)
	log.Printf("meta api: %s\n", url)
	return url
}

func (api *Api) GetAwsAPI(schema, apiName string) string {
	url := fmt.Sprintf("%s://%s/%s%s", schema, api.serverAddr, AwsAPIPath, apiName)
	log.Printf("aws api: %s\n", url)
	return url
}

type urlQuery struct {
	name  string
	value string
}

/// we will make sure the calling parameteres are even number
func pack(names ...string) []urlQuery {
	if len(names) == 0 {
		return []urlQuery{}
	}
	result := make([]urlQuery, 0, len(names)/2)
	k := ""
	for i, name := range names {
		if (i & 1) != 0 {
			result = append(result, urlQuery{k, name})
		} else {
			k = name
		}
	}
	return result
}

func (api *Api) DoPost(apiname string, reader io.Reader, queries []urlQuery) (*http.Response, error) {

	// FIXME: Most of the parameters are passed through "post", but actually using URL query.
	// It's bad practice, but we have to modify the server for all these changes. Not worth
	// it given the time budget at the moment.
	req, err := http.NewRequest(http.MethodPost, api.GetAPI("http", apiname), reader)
	if err != nil {
		log.Printf("error in constructing request: %v\n", err)
		return nil, err
	}
	if len(queries) > 0 {
		query := req.URL.Query()
		for _, q := range queries {
			query.Add(q.name, q.value)
		}
		req.URL.RawQuery = query.Encode()
	}
	return api.client.Do(req)
}

func (api *Api) DoGet(apiname string, queries []urlQuery) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, api.GetAPI("http", apiname), nil)
	if err != nil {
		log.Printf("error in constructing request: %v\n", err)
		return nil, err
	}
	if len(queries) > 0 {
		query := req.URL.Query()
		for _, q := range queries {
			query.Add(q.name, q.value)
		}
		req.URL.RawQuery = query.Encode()
	}
	return api.client.Do(req)
}

func (api *Api) DoAwsGet(apiname string, queries []urlQuery) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, api.GetAwsAPI("http", apiname), nil)
	if err != nil {
		log.Printf("error in constructing request: %v\n", err)
		return nil, err
	}
	if len(queries) > 0 {
		query := req.URL.Query()
		for _, q := range queries {
			query.Add(q.name, q.value)
		}
		req.URL.RawQuery = query.Encode()
	}
	return api.client.Do(req)
}

func (api *Api) UploadVmImage(name, location, gitrepo, gitrev, format string, encoded bool) error {

	/// read the data first and convert it to
	var (
		encoder *Base64FileReader
		err     error
	)

	if encoded {
		encoder, err = NewBase64FileReader(location)
	} else {
		encoder, err = NewBinaryFileReader(location)
	}

	if err != nil {
		log.Printf("error in opening the image file %s: %v\n", location, err)
		return err
	}

	defer encoder.Close()

	/// upload image and wait for response
	resp, err := api.DoPost(kUploadVmImage, encoder,
		pack(qImageGitRepo, gitrepo,
			qImageGitRev, gitrev,
			qImageName, name,
			qImageDiskFormat, format))
	if err != nil {
		fmt.Printf("error in uploading image: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) MyId() (string, error) {
	resp, err := api.DoGet(kViewPrincipalName, pack())
	if err != nil {
		fmt.Printf("error in view principal ID: %v\n", err)
		return "", err
	}
	return strResp(resp)
}

func (api *Api) MyNs() (string, error) {
	resp, err := api.DoGet(kViewNs, pack())
	if err != nil {
		fmt.Printf("error in view NS ID: %v\n", err)
		return "", err
	}
	return strResp(resp)
}

func (api *Api) CreatePrincipal(name string) error {
	resp, err := api.DoPost(kCreatePrincipal, nil, pack(qPrincipalName, name))
	if err != nil {
		fmt.Printf("error in creating principal: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) ListPrincipals() ([]string, error) {
	resp, err := api.DoGet(kListPrincipals, pack())
	if err != nil {
		fmt.Printf("error in listing principals: %v\n", err)
		return []string{}, err
	}
	return jsonResp(resp)
}

func (api *Api) DeletePrincipal(name string) error {
	resp, err := api.DoPost(kDeletePrincipal, nil, pack(qPrincipalName, name))
	if err != nil {
		fmt.Printf("error in deleting principal: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) CreateNs(ns string) error {
	resp, err := api.DoPost(kCreateNs, nil, pack(qNsName, ns))
	if err != nil {
		fmt.Printf("error in creating ns: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) JoinNs(ns string) error {
	resp, err := api.DoPost(kJoinNs, nil, pack(qNsName, ns))
	if err != nil {
		fmt.Printf("error in joining ns: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) LeaveNs(ns string) error {
	resp, err := api.DoPost(kLeaveNs, nil, pack(qNsName, ns))
	if err != nil {
		fmt.Printf("error in leaving ns: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) DeleteNs(ns string) error {
	resp, err := api.DoPost(kDeleteNs, nil, pack(qNsName, ns))
	if err != nil {
		fmt.Printf("error in leaving ns: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) CreateIPAlias(name string, ns string, ip net.IP) error {
	resp, err := api.DoPost(kCreateIPAlias, nil, pack(qNsName, ns,
		qPrincipalName, name, qIpAlias, ip.String()))
	if err != nil {
		fmt.Printf("error in creating Ip alias: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) DeleteIPAlias(name string, ns string, ip net.IP) error {
	resp, err := api.DoPost(kDeleteIPAlias, nil, pack(qNsName, ns,
		qPrincipalName, name, qIpAlias, ip.String()))
	if err != nil {
		fmt.Printf("error in deleting Ip alias: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) CreatePortAlias(name string, ns string, ip net.IP, protocol string,
	portMin, portMax int) error {
	resp, err := api.DoPost(kCreatePortAlias, nil, pack(qNsName, ns,
		qPrincipalName, name, qIpAlias, ip.String(), qProtocol, protocol,
		qPortMin, fmt.Sprintf("%d", portMin), qPortMax, fmt.Sprintf("%d", portMax)))
	if err != nil {
		fmt.Printf("error in creating port alias: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) DeletePortAlias(name string, ns string, ip net.IP, protocol string,
	portMin, portMax int) error {
	resp, err := api.DoPost(kDeletePortAlias, nil, pack(qNsName, ns,
		qPrincipalName, name, qIpAlias, ip.String(), qProtocol, protocol,
		qPortMin, fmt.Sprintf("%d", portMin), qPortMax, fmt.Sprintf("%d", portMax)))
	if err != nil {
		fmt.Printf("error in deleting port alias: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) postProof(target string, statements []Statement, apiname string) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(statements); err != nil {
		fmt.Printf("error in encoding statements: %v\n", err)
		return err
	}

	b64Statements := base64.StdEncoding.EncodeToString(buf.Bytes())
	resp, err := api.DoPost(apiname, nil, pack(qTarget, target,
		qStatements, b64Statements))
	if err != nil {
		fmt.Printf("error in posting proofs: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) PostProof(target string, statements []Statement) error {
	return api.postProof(target, statements, kPostProof)
}

func (api *Api) PostProofForChild(target string, statements []Statement) error {
	return api.postProof(target, statements, kPostProofForChild)
}

func (api *Api) linkProof(target string, dependencies []string, apiname string) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(dependencies); err != nil {
		fmt.Printf("error in encoding statements: %v\n", err)
		return err
	}

	b64Dependencies := base64.StdEncoding.EncodeToString(buf.Bytes())
	resp, err := api.DoPost(apiname, nil, pack(qTarget, target,
		qDependencies, b64Dependencies))
	if err != nil {
		fmt.Printf("error in linking proofs: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) LinkProof(target string, dependencies []string) error {
	return api.linkProof(target, dependencies, kLinkProof)
}

func (api *Api) LinkProofForChild(target string, dependencies []string) error {
	return api.linkProof(target, dependencies, kLinkProofForChild)
}

func (api *Api) SelfCertify(statements []Statement) error {
	buf := bytes.NewBuffer(make([]byte, 0))
	encoder := json.NewEncoder(buf)
	if err := encoder.Encode(statements); err != nil {
		fmt.Printf("error in encoding statements: %v\n", err)
		return err
	}

	b64Statements := base64.StdEncoding.EncodeToString(buf.Bytes())
	resp, err := api.DoPost(kSelfCertify, nil, pack(qStatements, b64Statements))
	if err != nil {
		fmt.Printf("error in posting proofs: %v\n", err)
		return err
	}
	return ok(resp)
}

func (api *Api) MyLocalIp() (string, error) {
	resp, err := api.DoAwsGet(kViewLocalIP, pack())
	if err != nil {
		fmt.Printf("error in view local IP: %v\n", err)
		return "", err
	}
	return strResp(resp)
}

func (api *Api) MyPublicIp() (string, error) {
	resp, err := api.DoAwsGet(kViewPublicIP, pack())
	if err != nil {
		fmt.Printf("error in view public IP: %v\n", err)
		return "", err
	}
	return strResp(resp)
}
