package docker

import "testing"

func TestLoadImage(t *testing.T) {

	image, err := LoadImage("../tests/", "a73140f6bc03aa8af6c958a18a4556cb1468ac90fda5ee91f7fafc0a7c0a76f3")
	if err != nil {
		t.Fatalf("err in loading image %v\n", err)
	}
	t.Logf("image source: %v\n", image.Source)
}
