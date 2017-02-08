package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	docker_image "github.com/docker/docker/image"
)

const (
	IMAGE_PATH         = "image/aufs"
	IMAGE_CONTENT_PATH = "image/aufs/imagedb/content/sha256/"
	IMAGE_REPO_FILE    = "repository.json"
	REPO_NAME          = "Repositories"
)

type Image struct {
	Versions map[string]string
}

type Repo struct {
	Images map[string]Image
}

type ParseError struct{ msg string }

func (e *ParseError) Error() string {
	return e.msg
}

func (i *Image) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &i.Versions); err != nil {
		fmt.Printf("error in parsing ImageVersions, %v\n", err)
		return err
	}
	return nil
}

func (r *Repo) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.Images); err != nil {
		fmt.Printf("error in parsing ImageRepo, %v\n", err)
		return err
	}
	return nil
}

func parseVersion(version string) (string, error) {
	/// Id is in form of "sha256:<real_id>"
	parts := strings.Split(version, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid canonical version %s", version)
	}
	return parts[1], nil
}

/* Defines the image metadata operations. */
func GetAllImageIds(r *Repo) []string {
	// format of a repo:
	//  'repo_name': {
	//      'version': 'sha256:id'
	//      ...
	//  }
	result := make([]string, 0, 2*len(r.Images))
	for _, images := range r.Images {
		for _, version := range images.Versions {
			id, err := parseVersion(version)
			if err != nil {
				fmt.Printf("parsing: %v\n", err)
			}
			result = append(result, id)
		}
	}
	return result
}

func imageRepoFile(root string) string {
	return path.Join(root, IMAGE_PATH, IMAGE_REPO_FILE)
}

func LoadImageRepos(root string) (*Repo, error) {
	repoFile, err := os.Open(imageRepoFile(root))
	if err != nil {
		log.Printf("can not open image repositories: %v\n", err)
		return nil, err
	}

	d := json.NewDecoder(repoFile)
	repo := &Repo{}
	if err := d.Decode(repo); err != nil {
		log.Printf("can not decode image repo config: %v\n", err)
		return nil, err
	}
	return repo, nil
}

func LoadImage(root, name string) (*docker_image.Image, error) {
	p := path.Join(root, IMAGE_CONTENT_PATH, name)
	content, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	image, err := docker_image.NewFromJSON(content)
	if err != nil {
		return nil, err
	}
	return image, nil
}
