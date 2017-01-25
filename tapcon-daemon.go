package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	pathutils "path"

	docker_types "github.com/docker/docker/container"
	"github.com/fsnotify/fsnotify"
	docker "github.com/jerryz920/tapcon-monitor/docker"
)

type ContainerMonitor struct {
	watcher    *fsnotify.Watcher
	path       string
	containers map[string]*docker_types.Container
}

func monitor_container_dir(path string) *fsnotify.Watcher {
	if watcher, err := fsnotify.NewWatcher(); err == nil {
		return watcher
	} else {
		log.Fatalln("can not create watcher: ", err)
		return nil
	}
}

func InitMonitor(path string) *ContainerMonitor {
	return &ContainerMonitor{
		watcher: monitor_container_dir(path),
		path:    path,
	}
}

func (m *ContainerMonitor) Close() {
	m.watcher.Close()
}

/// scan for existing containers, add to watch directory
func (m *ContainerMonitor) Scan() {
	if _, err := os.Stat(m.path); err != nil {
		log.Fatalf("container directory %s missing: %v\n", m.path, err)
	}

	if files, err := ioutil.ReadDir(m.path); err == nil {
		latest_containers := make(map[string]bool, 0, len(files))
		for _, f := range files {
			if !f.IsDir() {
				log.Printf("Warning: non-dir file in container root: %s", f.Name())
				continue
			}
			latest_containers[f.Name()] = true
		}
		m.SyncContainers(latest_containers)
	} else {
		log.Fatalln("error reading container root %s: %s", m.path, err)
	}

}

func (m *ContainerMonitor) SyncContainers(current map[string]bool) {

	to_delete = make([]string, 0, len(m.containers))
	to_create = make([]string, 0, len(m.containers))
	for c := range m.containers {
		if _, ok := current[c]; !ok {
			to_delete = append(to_delete, c)
		} else {
			delete(current, c)
		}
	}

	for c := range current {
		to_create = append(to_create, c)
	}

	m.CreateContainer(to_create)
	m.DeleteContainer(to_delete)
	m.ProcessUpdate()
}

func (m *ContainerMonitor) CreateContainer(containers []string) {
	for _, c := range containers {
		if new_container, err := docker.LoadContainer(c, m.path); err == nil {
			m.containers[c] = new_container
		} else {
			log.Printf("Error in loading container config for %s, delaying", c)
			// we do want to add it even if configure is not yet created.
			// here is a data race condition: if the DIR is created, but yet no
			// files written in, we might end up missing the event. So a safe
			// way is to wait for even anyway, and then have a pending list, in
			// order to ensure the containers are always collected correctly.
			m.containers[c] = nil
		}
		m.watcher.Add(pathutils.Join(c, m.path))
	}
}

func (m *ContainerMonitor) DeleteContainer(containers []string) {
	for _, c := range containers {
		delete(m.containers, c)
		m.watcher.Remove(pathutils.Join(c, m.path))
	}
}

func (m *ContainerMonitor) ProcessUpdate() {
	// in theory there shouldn't be more than 1000 containers on the host, so linear
	// scan the list would be quick. We make sure that all the containers in monitor
	// actually has a container config: which might be missing if we only listen to
	// inotify events

}

func (m *ContainerMonitor) SyncContainer(id string, path string) error {

	// read configuration file then append the container to
	// containers list
	container_root := pathutils.Join(path, id)
	if container, err := docker.LoadContainer(id, container_root); err == nil {
		m.containers[id] = container
		return nil
	} else {
		return err
	}
}

func do_something(monitor *ContainerMonitor, event fsnotify.Event) {

	// In order not to miss any event, first grab existing container
	// set and then compute difference with stored, then process
	// each container as if it is refreshed (Loaded). Old container
	// objects are replaced by new one and discarded in memory
	log.Println("event: ", event.String())

	// for each container load, try create the principal name, as
	// well as the alias names. Also, for each image built, mark
	// it with the image fact. Note the uniqueness of image ID
	// guarantees no matter where you store the image, it is that
	// very image you are working against.
}

func main() {
	flag.Parse()
	args := flag.Args()

	monitor := InitMonitor(args[0])
	defer monitor.Close()
	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-monitor.watcher.Events:
				do_something(monitor, event)
			case err := <-monitor.watcher.Errors:
				log.Println("error occurs ", err)
			}
		}
	}()

	<-done
}
