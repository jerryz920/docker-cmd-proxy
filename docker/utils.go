package docker

import (
	"log"
	"strings"
)

func tapconStringId(id string) string {
	if len(id) < ID_TRUNCATE_LEN {
		return id
	}
	return id[0:ID_TRUNCATE_LEN]
}

func tapconContainerId(c *MemContainer) string {
	return tapconStringId(c.Id)
}

func tapconContainerImageId(c *MemContainer) string {
	s := c.Config.ImageID.String()
	parts := strings.Split(s, ":")
	if len(parts) >= 2 {
		log.Printf("image parts %s\n", parts[1])
		return tapconStringId(parts[1])
	}
	log.Printf("image parts0 %s\n", parts[0])
	return tapconStringId(parts[0])
}

func tapconImageId(image *MemImage) string {
	return tapconStringId(image.Id)
}
