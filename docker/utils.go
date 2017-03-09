package docker

import (
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
		return tapconStringId(parts[1])
	}
	return tapconStringId(parts[0])
}

func tapconImageId(image *MemImage) string {
	return tapconStringId(image.Id)
}
