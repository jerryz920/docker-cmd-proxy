package types

import (
	"log"

	"github.com/docker/docker/container"
)

func LoadContainer(id string, root string) (*container.Container, error) {
	base_container := container.NewBaseContainer(id, root)
	if err := base_container.FromDisk(); err != nil {
		log.Print("error in loading the container content: ", err)
		return nil, err
	}
	return base_container, nil
}
