package daemon

import (
	"io/ioutil"
	"log"
	"os"
	filepath "path/filepath"
	"time"

	docker "github.com/docker/docker/container"
	docker_image "github.com/docker/docker/image"
	"github.com/fsnotify/fsnotify"
	tapcon_config "github.com/jerryz920/tapcon-monitor/config"
	tapcon_container "github.com/jerryz920/tapcon-monitor/docker"
)

type ContainerEvent int

const (
	CONTAINER_CREATED ContainerEvent = 1
	CONTAINER_REMOVED                = 2
	CONTAINER_UPDATED                = 3
	CONTAINER_NONE                   = 4
)

type ContainerMonitor struct {
	Watcher      *fsnotify.Watcher
	ImageWatcher *fsnotify.Watcher
	Path         string
	Containers   map[string]*docker.Container
	Repo         *tapcon_container.Repo
	Images       map[string]*docker_image.Image
	LastUpdate   time.Time
	timeout      time.Duration
}

func monitorDir(path string) *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("can not create watcher: %v\n", err)
	}
	if err := watcher.Add(path); err != nil {
		log.Fatalf("can not monitor path %s\n", path)
	}
	return watcher
}

func InitMonitor(path string, image_path string) *ContainerMonitor {
	clean_path, err := filepath.Abs(path)
	if err != nil {
		log.Fatalf("can not convert the path %s to absolute path: %v\n",
			path, err)
	}
	clean_image_path, err := filepath.Abs(image_path)

	return &ContainerMonitor{
		Watcher:     monitorDir(clean_path),
		ImageWacher: monitorDir(clean_image_path),
		Path:        clean_path,
		LastUpdate:  time.Now(),
		Containers:  make(map[string]*docker.Container),
		timeout:     tapcon_config.Config.Daemon.Timeout * time.Second,
	}
}

func (m *ContainerMonitor) Close() {
	m.Watcher.Close()
}

/// scan for existing containers, add to watch directory
func (m *ContainerMonitor) Scan() {
	if _, err := os.Stat(m.Path); err != nil {
		log.Fatalf("container directory %s missing: %v\n", m.Path, err)
	}

	if files, err := ioutil.ReadDir(m.Path); err == nil {
		latest_containers := make(map[string]bool)
		for _, f := range files {
			if !f.IsDir() {
				log.Printf("Warning: non-dir file in container root: %s", f.Name())
				continue
			}
			latest_containers[f.Name()] = true
		}
		m.SyncContainers(latest_containers)
	} else {
		log.Fatalln("error reading container root %s: %s", m.Path, err)
	}

	m.ReloadImageRepo()
}

func (m *ContainerMonitor) ReloadImageRepo() {

	m.Repo = tapcon_container.LoadImageRepos(m.Path)
}

func (m *ContainerMonitor) SyncContainers(current map[string]bool) {

	to_delete := make([]string, 0, len(m.Containers))
	to_create := make([]string, 0, len(m.Containers))
	for c := range m.Containers {
		if _, ok := current[c]; !ok {
			to_delete = append(to_delete, c)
		} else {
			delete(current, c)
		}
	}

	for c := range current {
		to_create = append(to_create, c)
	}

	update_time := time.Now()
	m.CreateContainer(to_create...)
	m.DeleteContainer(to_delete...)
	m.ProcessUpdate()
	m.LastUpdate = update_time
}

func (m *ContainerMonitor) CreateContainer(containers ...string) {
	for _, c := range containers {
		/// load container later in ProcessUpdate
		m.Containers[c] = nil
	}
}

func (m *ContainerMonitor) DeleteContainer(containers ...string) {
	for _, c := range containers {
		log.Printf("container %s removed\n", c)
		delete(m.Containers, c)
		//
		// fsnotify has a bug that can lead to dead lock on Remove. It will in fact
		// handle remove internally when a file is deleted. So we just let fsnotify
		// to address the remove.
		// Further, the removal of a watching file will lead to inotify descriptor
		// expire I believe. So don't worry about it.
		//
		//if err := m.Watcher.Remove(pathutils.Join(m.Path, c)); err != nil {
		//	log.Printf("something wrong with remove %s: %v", c, err)
		//}
	}
}

func (m *ContainerMonitor) ProcessUpdate() {
	// in theory there shouldn't be more than 1000 containers on the host, so linear
	// scan the list would be quick. We make sure that all the containers in monitor
	// actually has a container config: which might be missing if we only listen to
	// inotify events
	for id, c := range m.Containers {
		/// load container later in ProcessUpdate
		root_path := filepath.Join(m.Path, id)
		m.probeContainer(id, root_path, c == nil)
	}
}

func (m *ContainerMonitor) probeContainer(id, root string, force bool) error {
	if loaded, err := tapcon_container.LoadContainer(id, root, m.LastUpdate,
		force); err != nil {
		return err
	} else if loaded != nil {
		// Now configurations are in place, load them already. It may still
		// race if docker already deletes the configure path... But will it?
		for _, conf := range tapcon_container.ContainerConfigPaths(root) {
			if err := m.Watcher.Add(conf); err != nil {
				log.Printf("error watching configuration %s: %v", conf, err)
				return err
			}
		}
		m.Containers[id] = loaded
		log.Printf("container %s loaded", id)
	}
	return nil
}

func (m *ContainerMonitor) inspect() {
	log.Println("Inspecting monitor: \n")
	for _, c := range m.Containers {
		tapcon_container.ContainerInspect(c)
	}
}

func (m *ContainerMonitor) isContainerPath(p string) bool {
	dirname := filepath.Dir(p)
	return dirname == m.Path
}

func (m *ContainerMonitor) getContainerId(p string) string {
	if m.isContainerPath(p) {
		return filepath.Base(p)
	} else {
		/// it is in <m.path>/<id>/<file>
		container_path := filepath.Dir(p)
		return filepath.Base(container_path)
	}
}

func (m *ContainerMonitor) translateFsEvent(
	event fsnotify.Event) ContainerEvent {
	if m.isContainerPath(event.Name) {
		// the only special thing is 'create' and 'remove'
		// of the container directory. Either case the container
		// needs to present or be removed. For other cases, we
		// just assume the container is updated.
		switch event.Op {
		case fsnotify.Create:
			return CONTAINER_CREATED
		case fsnotify.Remove:
			return CONTAINER_REMOVED
		}
	}
	return CONTAINER_UPDATED
}

func (m *ContainerMonitor) handleContainerEvent(event fsnotify.Event) {
	log.Printf("processing event %s\n", event.String())
	container_id := m.getContainerId(event.Name)
	switch container_event := m.translateFsEvent(event); container_event {
	case CONTAINER_CREATED:
		m.CreateContainer(container_id)
		// At the time of config directory creation, it may already creates the config
		// file, and we try to probe it. (such event may lose anyway if the file is
		// created before we add it to watch list)
		if err := m.probeContainer(container_id, filepath.Join(m.Path, container_id),
			true); err != nil {
			log.Printf("fail to probe container, delay probing at scanning time")
		}
	case CONTAINER_REMOVED:
		// just delete it, assuming create/delete order is reversed at event delivering
		// time, we would eventually become consistent at scan time, it's just a little
		// bit slower than immediate consistency
		m.DeleteContainer(container_id)
	case CONTAINER_UPDATED:
		// For such event we force a container load. It can fail anyway due to
		// intermediate state. We will proceed to remove it eventually at
		// CONTAINER_REMOVED event. This would have a very small data race
		// window though: the state in monitor still says container is alive,
		// while container controller removes it already, leaving
		// configure directory not removed. Such temp state may hold util the
		// full configure path is removed. The reason we do so is because
		// docker is doing "atomic write" trick: write a temp config and rename
		// it, which will show reader either old or new version. This is
		// troublesome with fsnoitify, which does not really handle rename
		// well. To avoid the trouble, we endure a little bit of race. Or if
		// docker removes configure directory first then it removes the
		// container, we won't face such race.
		if err := m.probeContainer(container_id, filepath.Join(m.Path, container_id),
			true); err != nil {
			log.Printf("fail to probe container, system might be cleaning it up, skip")
		}
	}
}

func (m *ContainerMonitor) handleImageEvent(event fsnotify.Event) {

}

func (m *ContainerMonitor) WaitForEvent(sigchan chan os.Signal) {
	for {
		select {
		case <-time.After(m.timeout):
			m.Scan()
		case event := <-m.Watcher.Events:
			m.handleContainerEvent(event)
		case event := <-m.ImageWatcher.Events:
			m.handleImageEvent(event)
		case <-sigchan:
			m.inspect()
		case err := <-m.Watcher.Errors:
			log.Println("error occurs ", err)
		case err := <-m.ImageWatcher.Errors:
			log.Println("image error occurs", err)
		}
	}
}
