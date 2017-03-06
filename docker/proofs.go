package docker

import (
	"fmt"
	"log"

	metadata "github.com/jerryz920/tapcon-monitor/statement"
)

func (m *Monitor) PostImageProof(image *MemImage) error {
	id := tapconImageId(image)
	imageFact := metadata.Statement(
		fmt.Sprintf("imageFact(\"%s\", \"%s\", \"%s\", \"\", \"\")",
			id, image.Config.Source.Repo,
			image.Config.Source.Revision))
	return m.MetadataApi.PostProof(id, []metadata.Statement{imageFact})
}

func (m *Monitor) PostContainerFact(c *MemContainer) error {
	facts := c.ContainerFacts()
	cid := tapconContainerId(c)
	if len(facts) > 0 {
		return m.MetadataApi.PostProofForChild(cid, c.ContainerFacts())
	}
	log.Printf("no fact to post for container")
	return nil
}

func (m *Monitor) LinkContainerImage(c *MemContainer) error {
	cid := tapconContainerId(c)
	iid := tapconContainerImageId(c)
	return m.MetadataApi.LinkProofForChild(cid, []string{iid})
}
