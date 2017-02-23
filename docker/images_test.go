package docker

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadingRepo(t *testing.T) {
	root, err := filepath.Abs("../tests/image/aufs")
	if err != nil {
		t.Fatal("can not obtain abs path to test image repo")
	}
	repo, err := LoadImageRepos(root)
	assert.Equal(t, len(repo.Images), 7, "pre-set image repo count")
}

func TestGetAllImages(t *testing.T) {
	root, err := filepath.Abs("../tests/image/aufs")
	if err != nil {
		t.Fatal("can not obtain abs path to test image repo")
	}
	repo, err := LoadImageRepos(root)

	images := GetAllImageIds(repo)
	sort.Strings(images)
	expected := []string{
		"88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd",
		"88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd",
		"683a23d7da09f4da779babb8f1e11f9743080efab529857705ead20b3f9da762",
		"7968321274dc6b6171697c33df7815310468e694ac5be0ec03ff053bb135e768",
		"7968321274dc6b6171697c33df7815310468e694ac5be0ec03ff053bb135e768",
		"19134a8202e737105f1b53da5749afdda404c8926eccfcfc3dad2d6866d6d60c",
		"19134a8202e737105f1b53da5749afdda404c8926eccfcfc3dad2d6866d6d60c",
		"77cfa6ba4afdadca85d096c2469816b40541376ecb70d8c526095b741df2cf6a",
		"a39777a1a4a6ec8a91c978ded905cca10e6b105ba650040e16c50b3e157272c3",
		"a39777a1a4a6ec8a91c978ded905cca10e6b105ba650040e16c50b3e157272c3",
		"a73140f6bc03aa8af6c958a18a4556cb1468ac90fda5ee91f7fafc0a7c0a76f3",
	}
	sort.Strings(expected)
	assert.Equal(t, images, expected, "pre-set images")
}
