package manifestservice

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var ManifestDir = "data/manifests"

type ManifestService struct {
	// Define fields here
}

func New() *ManifestService {
	return &ManifestService{}
}

func (svc *ManifestService) CreateManifest(data []byte, repo, ref string) (string, error) {
	if err := os.MkdirAll(filepath.Join(ManifestDir, repo), 0755); err != nil {
		return "", err
	}

	manifestPath := filepath.Join(ManifestDir, repo, ref)
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return "", err
	}

	file, err := os.Open(manifestPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	haser := sha256.New()
	if _, err := io.Copy(haser, file); err != nil {
		return "", err
	}

	digest := fmt.Sprintf("sha256:%x", haser.Sum(nil))
	os.WriteFile(path.Join(ManifestDir, repo, digest[len("sha256:"):]), data, 0644)
	return digest, nil
}

func (svc *ManifestService) GetManifest(repo, ref string) ([]byte, string, error) {
	ref = ensureNoShaPrefix(ref)
	manifestPath := filepath.Join(ManifestDir, repo, ref)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	hasher := sha256.New()
	if _, err := hasher.Write(data); err != nil {
		return nil, "", err
	}

	return data, fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}

func ensureNoShaPrefix(digest string) string {
	if strings.HasPrefix(digest, "sha256:") {
		return digest[len("sha256:"):]
	}
	return digest
}
