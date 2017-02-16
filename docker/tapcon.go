package docker

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

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
	Containers             atomic.Value // map[string]*MemContainer
	Images                 atomic.Value // map[string]*MemImage
	Repo                   *Repo
	LastUpdate             time.Time
	timeout                time.Duration
	postMortemHandler      func(string)
	staticPortMin          int
	staticPortMax          int
	staticPortPerContainer int
	publicIp               net.IP
	localIp                net.IP
	localNs                string

	// port management for default network, no need to manage ports for
	// overlay network
	availableStaticPorts []bool

	tcpPorts map[string][]PortRange
	udpPorts map[string][]PortRange
}

func NewMonitor(containerRoot string, timeout time.Duration) (*Monitor, error) {

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
		Watcher:     watcher,
		MetadataApi: metadata_api.NewOpenstackMetadataAPI(""),
		//MetadataApi:         &metadata_api.StubApi{},
		CommandChan:         make(chan int),
		ContainerUpdateChan: make(chan *MemContainer, 10),
		SandboxBuilder:      &sandbox{},
		//SandboxBuilder:         &fakeSandbox{},
		ContainerMetadataPath:  containerPath,
		ContainerLock:          &sync.Mutex{},
		ImageMetadataPath:      imagePath,
		ImageLockCounter:       &sync.Mutex{},
		Networks:               make([]string, 0),
		NetworkWorkerQueue:     make([]NetworkDelayFunc, 0),
		NetworkWorkerLock:      &sync.Mutex{},
		LastUpdate:             time.Now(),
		timeout:                timeout,
		staticPortMin:          tapcon_config.Config.StaticPortBase,
		staticPortMax:          tapcon_config.Config.StaticPortMax,
		staticPortPerContainer: tapcon_config.Config.PortPerContainer,
	}
	m.Containers.Store(make(map[string]*MemContainer))
	m.Images.Store(make(map[string]*MemImage))

	m.availableStaticPorts = make([]bool, (m.staticPortMax-m.staticPortMin)/
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

func (m *Monitor) staticPortSlotAllocated(i int) bool {
	return m.availableStaticPorts[i]
}

func (m *Monitor) deallocateStaticPort(i int) {
	m.availableStaticPorts[i] = false
}

func (m *Monitor) nStaticPortSlot() int {
	return (m.staticPortMax - m.staticPortMin) / m.staticPortPerContainer
}

func (m *Monitor) resetAllStaticPortSlot() {
	maxSlot := m.nStaticPortSlot()
	for i := 0; i < maxSlot; i++ {
		m.availableStaticPorts[i] = false
	}
}

func (m *Monitor) PostImageProof(image *MemImage) error {
	imageFact := fmt.Sprintf("imageFact(\"%s\", \"%s\", \"%s\", \"\", \"\")",
		image.Config.ID().String(), image.Config.Source.Repo, image.Config.Source.Revision)
	encoded := metadata_api.Statement(base64.StdEncoding.EncodeToString([]byte(imageFact)))
	return m.MetadataApi.PostProof(image.Config.ID().String(), []metadata_api.Statement{encoded})
}

func (m *Monitor) PostContainerFact(c *MemContainer) error {
	containerFact := fmt.Sprintf("containerFact(\"%s\", \"%s\")", c.Id, c.Config.ImageID.String())
	encoded := metadata_api.Statement(base64.StdEncoding.EncodeToString([]byte(containerFact)))
	return m.MetadataApi.PostProof(c.Id, []metadata_api.Statement{encoded})

}

func (m *Monitor) allocateStaticPortSlot() (PortRange, error) {
	maxSlot := m.nStaticPortSlot()
	for i := 0; i < maxSlot; i++ {
		if !m.availableStaticPorts[i] {
			m.availableStaticPorts[i] = true
			min := m.staticPortMin + i*m.staticPortPerContainer
			max := m.staticPortMin + (i+1)*m.staticPortPerContainer - 1
			return PortRange{min: min, max: max}, nil
		}
	}
	return PortRange{0, 0}, fmt.Errorf("can not find available slot")
}

func (m *Monitor) scanImageUpdate() error {

	r, err := LoadImageRepos(m.ImageMetadataPath)
	if err != nil {
		log.Printf("error in loading repository file: %v\n", err)
		return err
	}
	m.Repo = r
	/// FIXME: may need to handle images not valid, but still in repositories.json
	images := GetAllImageIds(m.Repo)
	curImages := m.Images.Load().(map[string]*MemImage)
	newImages := make(map[string]*MemImage)
	needUpdate := len(images) == len(curImages)
	for _, id := range images {
		image, ok := curImages[id]
		if !ok {
			image = NewMemImage(m.ImageMetadataPath, id)
			needUpdate = true
		}
		if image.Config == nil {
			if err := image.Load(); err != nil {
				log.Printf("error in loading image %s: %v\n", id, err)
				continue
			}
			// Load tapcon principal
			/// Post the image Proofs
			if image.Config.Source.Repo == "" {
				image.IsTapconImage = false
			} else {
				if err := m.PostImageProof(image); err != nil {
					log.Printf("can't post proof for %s: %v\n", id, err)
					continue
				}
			}
		}
		newImages[id] = image
	}

	if needUpdate {
		m.Images.Store(newImages)
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
	oldContainers := m.Containers.Load().(map[string]*MemContainer)
	newContainers := make(map[string]*MemContainer)
	if create {
		for k, v := range oldContainers {
			newContainers[k] = v
		}
		root := filepath.Join(m.ContainerMetadataPath, id)
		tmp := NewMemContainer(id, root)
		newContainers[id] = tmp
		m.Watcher.Add(root)
		/// We may have lost fs event at the moment we start to watch the dir
		//m.ContainerUpdateChan <- tmp
		m.doContainerContentUpdate(tmp)
	} else {
		for k, v := range oldContainers {
			if k != id {
				newContainers[k] = v
			}
		}
	}
	m.Containers.Store(newContainers)
}

func (m *Monitor) containerEntryContentUpdate(id string) {
	// We have to load the config every time there is config changes, because
	// it can be container stop.
	containers := m.Containers.Load().(map[string]*MemContainer)
	c := containers[id]
	m.doContainerContentUpdate(c)
}

func (m *Monitor) doContainerContentUpdate(c *MemContainer) {
	wasRunning := c.Running()
	if c.Load() {
		log.Printf("container %s loaded\n", c.Id)
		// do the tapcon related update
		if wasRunning && !c.Running() {
			if err := m.MetadataApi.DeletePrincipal(c.Id); err != nil {
				log.Printf("error deleting principal %s: %v", c.Id, err)
			}
			/// Clear IPtables
			if err := m.SandboxBuilder.RemoveContainerChain(c.Id); err != nil {
				log.Printf("removing container chain %s: %v", c.Id, err)
			}
		} else if c.Running() {
			/*
			   step1: create principal

			   step2: create any ip alias

			   step3: assign static ports, create port alias, update Iptable rules

			   step4: link proofs of images

			*/
			if !c.PrincipalCreated {
				if err := m.MetadataApi.CreatePrincipal(c.Id); err != nil {
					log.Printf("error in creating principal %s: %v\n", c.Id, err)
					return
				}
				c.PrincipalCreated = true
			}

			if !c.ImageLinked {
				if err := m.PostContainerFact(c); err != nil {
					log.Printf("error in posting principal fact %s: %v\n", c.Id, err)
					return
				}
				if err := m.MetadataApi.LinkProof(c.Id,
					[]string{c.Config.ImageID.String()}); err != nil {
					log.Printf("error in linking principal %s: %v\n", c.Id, err)
					return
				}
				c.ImageLinked = true
			}

			for _, ip := range c.Ips {
				if ip == LOCALHOST_V4 {
					continue
				}
				nsName, err := c.GetNsName(ip)
				if err != nil {
					// we only create alias name for the overlay network
					continue
				}
				// we only need to create ip alias for overlay network
				isOverlay := false
				for _, overlay := range m.Networks {
					if overlay == nsName {
						isOverlay = true
					}
				}
				if !isOverlay {
					continue
				}
				created := false
				for _, name := range c.CreatedIpAliases {
					if name == nsName {
						created = true
						break
					}
				}
				if created {
					continue
				}

				if err := m.MetadataApi.CreateIPAlias(c.Id, nsName,
					net.ParseIP(ip)); err != nil {
					log.Printf("%v\n", err)
					continue
				}
				c.CreatedIpAliases = append(c.CreatedIpAliases, nsName)
			}

			if !c.PortAliasCreated && !c.Config.Config.NetworkDisabled {
				for _, hostBindings := range c.Config.NetworkSettings.Ports {
					//var cport int = int(strconv.ParseUint(containerPort, 10, 0))
					for _, binding := range hostBindings {
						tmp, err := strconv.ParseUint(binding.HostPort, 10, 0)
						if err != nil {
							log.Printf("can not parse host port: %s\n", binding.HostPort)
							continue
						}
						hport := int(tmp)
						// Let's make it simple: exposed ports only for the local and public
						// IPs of the instance, not the overlayed network
						m.setupPortMapping(c, hport, hport)
						// we assume we have only one binding for any time atm
						break
					}
				}

				/// Assign ports
				// pmin, pmax = m.AllocatePorts
				// setupIpTable
				// createPortAlias()
				//
				prange, err := m.allocateStaticPortSlot()
				if err != nil {
					log.Printf("can not allocate static port range for it\n")
					return
				}
				c.StaticPortMin = prange.min
				c.StaticPortMax = prange.max
				m.setupPortMapping(c, prange.min, prange.max)

				m.SandboxBuilder.SetupContainerChain(c.Id)
				for _, ip := range c.Ips {
					if c.IsContainerIp(ip) {
						m.SandboxBuilder.SetupStaticPortMapping(c.Id, ip, prange.min, prange.max)
					}
				}
				c.PortAliasCreated = true
			}
		}

	} else {
		// if the config is cleared after the load process, it means there is
		// some error, e.g. container configs stopped. We need to clear the
		// tapcon principals.
		if wasRunning && !c.Running() {
			if err := m.MetadataApi.DeletePrincipal(c.Id); err != nil {
				log.Printf("error deleting principal %s: %v", c.Id, err)
			}
			/// Clear IPtables
			if err := m.SandboxBuilder.RemoveContainerChain(c.Id); err != nil {
				log.Printf("removing container chain %s: %v", c.Id, err)
			}
		}
	}
}

func (m *Monitor) containerEntriesReload() {
	// There might be containers existing before the daemon actually starts, scan and
	// fill in them.
	files, err := ioutil.ReadDir(m.ContainerMetadataPath)
	if err != nil {
		log.Fatalf("error in reading container root: %v\n", err)
	}
	// for each containers, probe the container config
	oldContainers := m.Containers.Load().(map[string]*MemContainer)
	newContainers := make(map[string]*MemContainer)
	needUpdate := len(files) == len(oldContainers)
	for _, f := range files {
		if !f.IsDir() {
			log.Printf("Warning: non-dir file in container root: %s", f.Name())
			continue
		}
		fname := f.Name()
		if old, ok := oldContainers[fname]; ok {
			newContainers[fname] = old
			// we may need to update tapcon principals, do update in unified entrance
			m.doContainerContentUpdate(old)
		} else {
			needUpdate = true
			tmp := NewMemContainer(fname,
				filepath.Join(m.ContainerMetadataPath, fname))
			newContainers[fname] = tmp
			m.Watcher.Add(tmp.Root)
			m.doContainerContentUpdate(tmp)
		}
	}
	if needUpdate {
		m.Containers.Store(newContainers)
	}
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
		log.Printf("image update\n")
		m.ScanImageUpdate()
	} else if IsContainerPath(path, m.ContainerMetadataPath) {
		// It points to a container DIR. We do not consider the case where a directory
		// is created with "rename" or any rename event inside container path. However,
		// rename does happen for individual container configures, where we reload
		// container every time if it happens.
		log.Printf("new container path: %s\n", path)
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
			log.Printf("container config change: %s\n", fname)
			id := ContainerPathToId(path, m.ContainerMetadataPath)
			//go m.containerEntryContentUpdate(id)
			m.containerEntryContentUpdate(id)
		}
	}
	return nil
}

func (m *Monitor) ScanNetworkUpdate() {
	toAdd, toDelete := m.NetworkChanges()
	if len(toAdd) > 0 || len(toDelete) > 0 {
		log.Printf("debug: adding network: %v, deleting %v\n", toAdd, toDelete)

		for _, n := range toAdd {
			if err := m.MetadataApi.CreateNs(n); err != nil {
				log.Printf("failing to create ns %s, which may be created already\n", n)
			}
			if err := m.MetadataApi.JoinNs(n); err != nil {
				log.Printf("failing to join ns %s\n", n)
			}
		}

		for _, n := range toDelete {
			if err := m.MetadataApi.LeaveNs(n); err != nil {
				log.Printf("failing to leave ns %s\n", n)
			}
			if err := m.MetadataApi.DeleteNs(n); err != nil {
				log.Printf("failing to delete ns %s\n", n)
			}
		}
	}
}

func (m *Monitor) Scan() {
	m.ScanImageUpdate()
	m.ScanNetworkUpdate()
	m.containerEntriesReload()
}

func (m *Monitor) Dump() {
	log.Printf("current networks: %v\n", m.Networks)
	log.Printf("container path %s\n", m.ContainerMetadataPath)
	log.Printf("image path %s\n", m.ImageMetadataPath)
	log.Printf("timeout %v\n", m.timeout)
	log.Printf("static %d %d %d\n", m.staticPortMin, m.staticPortMax,
		m.staticPortPerContainer)
	log.Printf("ipinfo: %s %s %s\n", m.publicIp.String(), m.localIp.String(),
		m.localNs)
	result := make([]string, 0, len(m.availableStaticPorts))
	for i, p := range m.availableStaticPorts {
		if p {
			pmin := m.staticPortMin + i*m.staticPortPerContainer
			pmax := pmin + m.staticPortPerContainer - 1
			result = append(result, fmt.Sprintf("%d-%d", pmin, pmax))
		}
	}
	log.Printf("allocated ports: %v\n", result)
	log.Printf("****Containers\n")
	containers := m.Containers.Load().(map[string]*MemContainer)
	for _, c := range containers {
		c.Dump()
	}
	log.Printf("****Images\n")
	images := m.Images.Load().(map[string]*MemImage)
	for _, i := range images {
		i.Dump()
	}
}

func (m *Monitor) WorkAndWait(sigchan chan os.Signal) {

	for {
		select {
		case e := <-m.Watcher.Events:
			if err := m.handleFsEvent(e); err != nil {
				log.Printf("error handling event %v\n", err)
			}
		case c := <-m.ContainerUpdateChan:
			//go m.doContainerContentnUpdate(c)
			m.doContainerContentUpdate(c)
		case e := <-m.Watcher.Errors:
			log.Printf("error event: %s\n", e.Error())
			break
		case <-time.After(m.timeout):
			log.Printf("timeout %v\n", m.timeout)
			//go m.Scan()
			m.Scan()
		case <-sigchan:
			m.Dump()
		}
	}
}
