package types

import (
	"log"
	"os"
	"path"
	"time"

	"github.com/docker/docker/container"
)

const (
	CONTAINER_CONFIG_FILE = "config.v2.json"
	CONTAINER_HOST_CONFIG = "hostconfig.json"
)

func modifiedSinceLast(last_update time.Time, root string) (bool, error) {
	config_file := path.Join(root, CONTAINER_CONFIG_FILE)
	if result, err := os.Stat(config_file); err != nil {
		return false, err
	} else {
		return last_update.Before(result.ModTime()), nil
	}
}

func LoadContainer(id string, root string, last_update time.Time,
	force bool) (*container.Container, error) {
	if !force {
		if ok, err := modifiedSinceLast(last_update, root); err != nil || !ok {
			return nil, err
		}
	}

	base_container := container.NewBaseContainer(id, root)
	if err := base_container.FromDisk(); err != nil {
		log.Print("error in loading the container content: ", err)
		return nil, err
	}
	return base_container, nil
}

func ContainerInspect(c *container.Container) {
	log.Printf("----------------------")
	log.Printf("container id: %s, running: %v\n", c.ID, c.Running)
	log.Printf("container start at: %s, finish at: %s\n", c.StartedAt, c.FinishedAt)
	log.Printf("container ImageID: %v\n", c.ImageID)
	log.Printf("container config Image: %s", c.Config.Image)
}

func ContainerConfigPaths(root string) []string {
	return []string{path.Join(root, CONTAINER_CONFIG_FILE),
		path.Join(root, CONTAINER_HOST_CONFIG)}
}
