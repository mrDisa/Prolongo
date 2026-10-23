package openapi

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"
)

const rootSpec = "../../openapi.yml"

func TestCopyMatchesRoot(t *testing.T) {
	root, err := os.ReadFile(rootSpec)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("root openapi.yml is not available (tests run outside the repository)")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(root, Spec) {
		t.Fatal("backend-go/openapi/openapi.yml differs from the root openapi.yml: run `make openapi`")
	}
}

func TestSpecLooksLikeOpenAPI(t *testing.T) {
	if !bytes.HasPrefix(Spec, []byte("openapi: 3.1.0\n")) {
		t.Fatalf("unexpected spec start: %q", Spec[:min(40, len(Spec))])
	}
}
