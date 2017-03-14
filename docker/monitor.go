package docker

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/fsnotify/fsnotify"
	tapcon_config "github.com/jerryz920/tapcon-monitor/config"
	metadata_api "github.com/jerryz920/tapcon-monitor/statement"
)

/* logic:

when the container dir is created, create an entry in monitored principals
launch a go routine to monitor specific configuration file creation


*/

// container creation/deletion state
const (
	CONTAINER_CREATED int = 0
	CONTAINER_UPDATED
	CONTAINER_DELETED
	IMAGE_REPO_UPDATED
)

const (
	IMAGE_UPDATING = 1
	IMAGE_IDLE     = 0
)

const (
	ID_TRUNCATE_LEN = 13
)

type ServerContainerState struct {
	metadata_api.Principal
	lock *sync.Mutex
}

type Monitor struct {
	Watcher *fsnotify.Watcher

	MetadataApi            metadata_api.MetadataAPI
	CommandChan            chan int /// Should be a "command" in future
	ContainerUpdateChan    chan *MemContainer
	SandboxBuilder         Sandbox
	NetworkWorkerLock      *sync.Mutex
	Networks               []string /// current networks
	NetworkWorkerQueue     []NetworkDelayFunc
	ContainerMetadataPath  string
	ImageMetadataPath      string
	ImageLockCounter       *sync.Mutex
	ContainerLock          *sync.Mutex
	Containers             map[string]*MemContainer
	Images                 map[string]*MemImage
	Repo                   *Repo
	LastUpdate             time.Time
	timeout                time.Duration
	cache                  ReconcileCache
	reconcileTimeout       time.Duration
	postMortemHandler      func(string)
	staticPortMin          int
	staticPortMax          int
	staticPortPerContainer int
	publicIp               net.IP
	localIp                net.IP
	localNs                string
	debug                  bool

	// port management for default network, no need to manage ports for
	// overlay network
	availableStaticPorts []int32

	tcpPorts map[string][]PortRange
	udpPorts map[string][]PortRange
}

func NewMonitor(containerRoot string, api metadata_api.MetadataAPI,
	sbox Sandbox, debug bool) (*Monitor, error) {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("can not create fs monitor: %v\n", err)
	}
	containerRoot, err = filepath.Abs(containerRoot)
	if err != nil {
		log.Fatalf("can not obtain absolute directory: %v\n", err)
	}
	containerPath := filepath.Join(containerRoot, "containers")
	imagePath := filepath.Join(containerRoot, IMAGE_PATH)
	watcher.Add(imagePath)
	watcher.Add(containerPath)

	m := &Monitor{
		Watcher:                watcher,
		MetadataApi:            api,
		CommandChan:            make(chan int),
		ContainerUpdateChan:    make(chan *MemContainer, 10),
		SandboxBuilder:         sbox,
		ContainerMetadataPath:  containerPath,
		ContainerLock:          &sync.Mutex{},
		ImageMetadataPath:      imagePath,
		ImageLockCounter:       &sync.Mutex{},
		Containers:             make(map[string]*MemContainer),
		Images:                 make(map[string]*MemImage),
		Networks:               make([]string, 0),
		NetworkWorkerQueue:     make([]NetworkDelayFunc, 0),
		NetworkWorkerLock:      &sync.Mutex{},
		LastUpdate:             time.Now(),
		timeout:                tapcon_config.Config.Daemon.Timeout * time.Second,
		debug:                  debug,
		staticPortMin:          tapcon_config.Config.StaticPortBase,
		staticPortMax:          tapcon_config.Config.StaticPortMax,
		staticPortPerContainer: tapcon_config.Config.PortPerContainer,
	}
	if api == nil {
		m.MetadataApi = metadata_api.NewOpenstackMetadataAPI("")
	}
	if sbox == nil {
		m.SandboxBuilder = &sandbox{}
	}

	m.availableStaticPorts = make([]int32, (m.staticPortMax-m.staticPortMin)/
		m.staticPortPerContainer)
	m.resetAllStaticPortSlot()
	m.setupInstanceIpInfo()
	// Force a scan to avoid missing events
	m.Scan()

	// update the image for the first time. There might be duplicated event if
	// it happens to be modified during the first update. But it's not a problem as
	// Docker writes out config files in atomic way. There is no chance to see a
	// partial config
	return m, nil
}

func (m *Monitor) scanImageUpdate() error {

	r, err := LoadImageRepos(m.ImageMetadataPath)
	if err != nil {
		log.Errorf("loading repository file: %v", err)
		return err
	}
	m.Repo = r
	/// FIXME: may need to handle images not valid, but still in repositories.json
	images := GetAllImageIds(m.Repo)
	for _, id := range images {
		image, ok := m.Images[id]
		if !ok {
			image = NewMemImage(m.ImageMetadataPath, id)
		}
		if image.Config == nil {
			if err := image.Load(); err != nil {
				log.Errorf("loading image %s: %v", id, err)
				continue
			}
			// Load tapcon principal
			/// Post the image Proofs
			if err := m.PostImageProof(image); err != nil {
				log.Errorf("can't post proof for %s: %v", id, err)
				continue
			}
		}
		m.Images[id] = image
	}

	return nil
}

func (m *Monitor) ScanImageUpdate() error {
	m.ImageLockCounter.Lock()
	defer m.ImageLockCounter.Unlock()
	return m.scanImageUpdate()
}

func (m *Monitor) containerEntryUpdate(id string, create bool) {

	// we use immutable way to update the in memory handle. It is not a big
	// overhead to copy a hundred pointers.
	cid := tapconStringId(id)
	if create {
		root := filepath.Join(m.ContainerMetadataPath, id)
		m.AllocateNewMemContainer(cid, root)
	} else {
		m.ContainerLock.Lock()
		defer m.ContainerLock.Unlock()
		if c, ok := m.Containers[cid]; ok {
			log.Infof("removing container entry: %s", cid)
			delete(m.Containers, cid)
			c.EventChan <- CONTAINER_DEAD
		}
	}
}

func (m *Monitor) Keeper(c *MemContainer) {
	//cid := tapconContainerId(c)
	//m.SandboxBuilder.SetupContainerChain(cid)
	/// Apply restriction on container
	for {
		select {
		case e := <-c.EventChan:
			if e == CONTAINER_DEAD {
				break
			}
			c.Refresh()
			//if err := c.Refresh(); err != nil {
			//	// this may happen that a container has been deleted
			//	log.Printf("refresh error: %v", err)
			//}
			if c.Load() {
				//if c.StaticPortMin == 0 {
				//	prange, err := m.allocateStaticPortSlot()
				//	m.SandboxBuilder.ClearStaticPortMapping(cid)
				//	if err != nil {
				//		log.Warnf("unable to allocate static ports, retry later")
				//		continue
				//	}
				//	c.StaticPortMin = prange.min
				//	c.StaticPortMax = prange.max
				//	for _, ip := range c.Ips {
				//		if c.IsContainerIp(ip) {
				//			m.SandboxBuilder.SetupStaticPortMapping(cid, ip,
				//				prange.min, prange.max)
				//		}
				//	}
				//}
				/// no matter refresh success or fail, we will resync the
				// server cache (maybe empty) and client side status

				//set repo string
				/// Hotcloud2017Workaround
				log.Debugf("container %s loaded, reconciling", c.Id)
				c.Cache.Create()
			} else {
				log.Debugf("container %s removed, reconciling", c.Id)
				c.Cache.Remove()
				//m.SandboxBuilder.ClearStaticPortMapping(cid)
				//m.deallocateStaticPortByContainer(c)
			}
		}
	}
	//m.SandboxBuilder.RemoveContainerChain(cid)
	/// withdraw restriction on container
	//m.deallocateStaticPortByContainer(c)
}

func (m *Monitor) containerEntriesReload() {
	// There might be containers existing before the daemon actually starts, scan and
	// fill in them.
	files, err := ioutil.ReadDir(m.ContainerMetadataPath)
	if err != nil {
		log.Fatalf("error in reading container root: %v", err)
	}
	// for each containers, probe the container config
	/// Download the principal list, then do the update
	serverState, err := m.MetadataApi.ListPrincipals()
	if err != nil {
		log.Warnf("update stopped can not fetch server state: %v", err)
	}
	toDelete := make([]string, 0, len(files))

	m.ContainerLock.Lock()
	defer m.ContainerLock.Unlock()

	for _, f := range files {
		if !f.IsDir() {
			log.Warnf("non-dir file in container root: %s", f.Name())
			continue
		}
		cid := tapconStringId(f.Name())
		if c, ok := m.Containers[cid]; ok {
			c.EventChan <- NEED_UPDATE
		} else {
			root := filepath.Join(m.ContainerMetadataPath, f.Name())
			m.allocateNewMemContainer(cid, root)
		}
	}

	for cid, c := range m.Containers {
		found := false
		for _, f := range files {
			fcid := tapconStringId(f.Name())
			if cid == fcid {
				found = true
				break
			}
		}
		if !found {
			toDelete = append(toDelete, cid)
			c.EventChan <- CONTAINER_DEAD
		}
	}

	for _, cid := range toDelete {
		log.Infof("removing container entry: %s", cid)
		delete(m.Containers, cid)
	}

	if serverState != nil {
		for pname, _ := range serverState {
			found := false
			for _, f := range files {
				cid := tapconStringId(f.Name())
				if pname == cid {
					found = true
					break
				}
			}
			if !found {
				log.Infof("staled principal %s", pname)
				m.MetadataApi.DeletePrincipal(pname)
			}
		}
	}
}

func (m *Monitor) allocateNewMemContainer(id, root string) {
	c := NewMemContainer(id, root, m.localNs)
	/// This looks really ugly... fix it sometimes
	if m.debug {
		c.listIp = StubListIP
	}
	c.Cache = NewReconcileCache(m.MetadataApi, c)
	c.VmIps = []instanceIp{
		instanceIp{
			ns: m.localNs,
			ip: m.localIp.String(),
		},
		instanceIp{
			ns: DEFAULT_NS,
			ip: m.publicIp.String(),
		},
	}

	log.Infof("loading container entry: %s", id)

	m.Containers[id] = c
	m.Watcher.Add(root)
	go m.Keeper(c)
	c.EventChan <- NEED_UPDATE
}

func (m *Monitor) AllocateNewMemContainer(id, root string) {
	m.ContainerLock.Lock()
	m.allocateNewMemContainer(id, root)
	m.ContainerLock.Unlock()
}

/*
  fs events need to be translated to map container events:
    1. container dir create -> ContainerCreated
    2. container config created/host config changes:
    	state = Created, event = Created/Deleted host/config -> ContainerUpdate
		if probe fails, ignore. There could be multiple event for one
		file change, we don't care each event we try to update the in
		memory container config. It may duplicate, we can eliminate that
		by time stamp and inode
    3. container dir removed -> ContainerDeleted
*/
func (m *Monitor) handleFsEvent(e fsnotify.Event) error {

	path := e.Name
	/// Image repo update
	if path == m.ImageMetadataPath {
		//go m.ScanImageUpdate()
		log.Debugf("image update")
		m.ScanImageUpdate()
	} else if IsContainerPath(path, m.ContainerMetadataPath) {
		// It points to a container DIR. We do not consider the case where a directory
		// is created with "rename" or any rename event inside container path. However,
		// rename does happen for individual container configures, where we reload
		// container every time if it happens.
		log.Debugf("new container path: %s", path)
		id := ContainerPathToId(path, m.ContainerMetadataPath)
		switch e.Op {
		case fsnotify.Create:
			//		go m.containerEntryUpdate(id, true)
			m.containerEntryUpdate(id, true)
		case fsnotify.Remove:
			//go m.containerEntryUpdate(id, false)
			m.containerEntryUpdate(id, false)
			// There is no need to remove watcher as it will automatically be removed
			// when the OS remove the directory.
		default:
			// just skip the event
			return nil
		}
	} else {
		fname := filepath.Base(path)
		if ContainerConfigFile(fname) {
			log.Debugf("container config change: %s", fname)
			id := ContainerPathToId(path, m.ContainerMetadataPath)
			cid := tapconStringId(id)

			m.ContainerLock.Lock()
			defer m.ContainerLock.Unlock()
			c, ok := m.Containers[cid]
			if !ok {
				log.Debugf("container not found! Adding")
				m.allocateNewMemContainer(cid, filepath.Join(m.ContainerMetadataPath, id))
			} else {
				c.EventChan <- NEED_UPDATE
			}
		}
	}
	return nil
}

func (m *Monitor) ScanNetworkUpdate() {
	toAdd, toDelete := m.NetworkChanges()
	if len(toAdd) > 0 || len(toDelete) > 0 {
		log.Debugf("adding network: %v, deleting %v", toAdd, toDelete)

		for _, n := range toAdd {
			if err := m.MetadataApi.CreateNs(n); err != nil {
				log.Warning("failing to create ns %s, which may be created already", n)
			}
			if err := m.MetadataApi.JoinNs(n); err != nil {
				log.Errorf("failing to join ns %s", n)
			}
		}

		for _, n := range toDelete {
			if err := m.MetadataApi.LeaveNs(n); err != nil {
				log.Errorf("failing to leave ns %s", n)
			}
			if err := m.MetadataApi.DeleteNs(n); err != nil {
				log.Errorf("failing to delete ns %s", n)
			}
		}
	}
}

func (m *Monitor) Scan() {
	m.ScanImageUpdate()
	m.ScanNetworkUpdate()
	m.containerEntriesReload()
}

func (m *Monitor) WorkAndWait(sigchan chan os.Signal) {

	for {
		select {
		case e := <-m.Watcher.Events:
			if err := m.handleFsEvent(e); err != nil {
				log.Errorf("handling event %v", err)
			}
		case e := <-m.Watcher.Errors:
			log.Errorf("event: %s", e.Error())
			break
		case <-time.After(m.timeout):
			log.Errorf("timeout %v", m.timeout)
			go m.Scan()
		case <-time.After(m.reconcileTimeout):
			// fetch reconcile cache from server
			//m.Reconcile()
		case <-sigchan:
			m.Dump()
		}
	}
}

func (m *Monitor) Dump() {
	log.Infof("current networks: %v", m.Networks)
	log.Infof("container path %s", m.ContainerMetadataPath)
	log.Infof("image path %s", m.ImageMetadataPath)
	log.Infof("timeout %v", m.timeout)
	log.Infof("static %d %d %d", m.staticPortMin, m.staticPortMax,
		m.staticPortPerContainer)
	log.Infof("ipinfo: %s %s %s", m.publicIp.String(), m.localIp.String(),
		m.localNs)
	result := make([]string, 0, len(m.availableStaticPorts))
	for i, p := range m.availableStaticPorts {
		// This may have memory ordering issue as we didn't use barrier. But
		// it doesn't matter
		if p != 0 {
			pmin := m.staticPortMin + i*m.staticPortPerContainer
			pmax := pmin + m.staticPortPerContainer - 1
			result = append(result, fmt.Sprintf("%d-%d", pmin, pmax))
		}
	}
	log.Infof("allocated ports: %v", result)
	m.ContainerLock.Lock()
	log.Infof("-------Containers---------")

	for _, c := range m.Containers {
		c.Dump()
	}
	log.Infof("--------------------------")
	m.ContainerLock.Unlock()
	log.Infof("----------Images----------")
	m.ImageLockCounter.Lock()
	for _, i := range m.Images {
		i.Dump()
	}
	log.Infof("--------------------------")
	m.ImageLockCounter.Unlock()
}
