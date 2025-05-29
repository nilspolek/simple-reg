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
	tags map[string][]string
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

	svc.tags[repo] = append(svc.tags[repo], ref)

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

func (svc *ManifestService) DeleteManifest(repo, ref string) error {
	ref = ensureNoShaPrefix(ref)
	manifestPath := filepath.Join(ManifestDir, repo, ref)
	if err := os.Remove(manifestPath); err != nil {
		return err
	}
	tags := svc.tags[repo]

	// remove tag from tags
	for index, tag := range tags {
		if tag == ref {
			tags = append(tags[:index], tags[index+1:]...)
			break
		}
	}
	return nil
}

func ensureNoShaPrefix(digest string) string {
	if strings.HasPrefix(digest, "sha256:") {
		return digest[len("sha256:"):]
	}
	return digest
}

func (svc *ManifestService) GetAllTags() map[string][]string {
	if len(svc.tags) == 0 {
		svc.tags = loadTags()
	}

	return svc.tags
}

func (svc *ManifestService) GetTags(repo string) []string {
	if len(svc.tags) == 0 {
		svc.tags = loadTags()
	}

	if _, ok := svc.tags[repo]; !ok {
		svc.tags[repo] = []string{}
	}

	return svc.tags[repo]
}

func loadTags() map[string][]string {
	tags := make(map[string][]string)
	files, err := os.ReadDir(ManifestDir)
	if err != nil {
		return tags
	}

	// get each directory in ManifestDir
	folderNames := make([]string, 0)
	for _, file := range files {
		if file.IsDir() {
			folderNames = append(folderNames, file.Name())
		}
	}

	// get each tag in each directory
	for _, dir := range folderNames {
		files, err := os.ReadDir(filepath.Join(ManifestDir, dir))
		if err != nil {
			continue
		}
		for _, file := range files {
			if len(file.Name()) == 64 {
				// tag is a sha
				continue
			}
			tags[dir] = append(tags[dir], file.Name())
		}
	}

	return tags
}
