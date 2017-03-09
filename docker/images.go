package docker

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	docker_image "github.com/docker/docker/image"
)

const (
	IMAGE_PATH         = "image/aufs"
	IMAGE_CONTENT_PATH = "imagedb/content/sha256/"
	IMAGE_REPO_FILE    = "repositories.json"
	REPO_NAME          = "Repositories"
)

/// names are ugly, rename things later
type MemImage struct {
	Config        *docker_image.Image
	Id            string
	Hash          string // hash method, we assume it's sha256
	Root          string
	IsTapconImage bool
	Mutex         *sync.Mutex
}

type Image struct {
	Versions map[string]string
}

type Repo struct {
	Images     map[string]Image
	lastUpdate time.Time
}

type ParseError struct{ msg string }

func (e *ParseError) Error() string {
	return e.msg
}

func (i *Image) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &i.Versions); err != nil {
		log.Errorf("parsing ImageVersions, %v", err)
		return err
	}
	return nil
}

func (r *Repo) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.Images); err != nil {
		log.Errorf("parsing ImageRepo, %v", err)
		return err
	}
	return nil
}

func NewMemImage(root, id string) *MemImage {
	return &MemImage{
		Config:        nil,
		Id:            id,
		Hash:          "sha256",
		Root:          root,
		Mutex:         &sync.Mutex{},
		IsTapconImage: true,
	}
}

func (i *MemImage) Load() error {
	/// We don't reload the config
	i.Mutex.Lock()
	defer i.Mutex.Unlock()
	if !i.IsTapconImage {
		return nil
	}
	if i.Config != nil {
		return nil
	}
	conf, err := LoadImage(i.Root, i.Id)
	if err != nil {
		return err
	}
	i.Config = conf
	return nil
}

func (i *MemImage) Dump() {
	log.Infof("-----ImageId: %s", i.Id)
	log.Infof("root: %s", i.Root)
	log.Infof("Source: %v", i.Config.Source)
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
				fmt.Errorf("parsing: %v\n", err)
				continue
			}
			result = append(result, id)
		}
	}
	return result
}

func imageRepoFile(root string) string {
	return path.Join(root, IMAGE_REPO_FILE)
}

func NeedReload(r *Repo, imageRoot string) bool {
	repoFile, err := os.Open(imageRepoFile(imageRoot))
	defer repoFile.Close()
	if err != nil {
		log.Errorf("can not open image repositories: %v", err.Error())
		return false
	}
	stat, err := repoFile.Stat()
	if err != nil {
		log.Errorf("can not stat image repositories: %v", err.Error())
		return false
	}
	if r.lastUpdate.Before(stat.ModTime()) {
		return true
	}
	return false
}

func LoadImageRepos(imageRoot string) (*Repo, error) {
	repoFile, err := os.Open(imageRepoFile(imageRoot))
	defer repoFile.Close()
	if err != nil {
		log.Errorf("can not open image repositories: %v", err.Error())
		return nil, err
	}

	d := json.NewDecoder(repoFile)
	repos := make(map[string]*Repo)
	if err := d.Decode(&repos); err != nil {
		log.Errorf("can not decode image repo config: %v", err.Error())
		return nil, err
	}
	repo := repos[REPO_NAME]
	stat, err := repoFile.Stat()
	if err != nil {
		return nil, err
	}
	repo.lastUpdate = stat.ModTime()

	return repo, nil
}

func LoadImage(imageRoot, name string) (*docker_image.Image, error) {
	p := path.Join(imageRoot, IMAGE_CONTENT_PATH, name)
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
