package blobservice

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	DefaultBlobDir     = "data/blobs"
	DefaultManifestDir = "data/manifests"
)

var (
	ErrUploadSessionNotFound = errors.New("upload session not found")
	ErrInvalidHash           = errors.New("invalid hash")
	ErrManifestNotFound      = errors.New("manifest not found")
	ErrBlobNotFound          = errors.New("blob not found")
)

type BlobService struct {
	BlobDir        string
	ManifestDir    string
	UploadSessions map[uuid.UUID]*os.File
	uploadMx       sync.Mutex
}

func New() *BlobService {
	return &BlobService{
		BlobDir:        DefaultBlobDir,
		ManifestDir:    DefaultManifestDir,
		UploadSessions: make(map[uuid.UUID]*os.File),
		uploadMx:       sync.Mutex{},
	}
}

func (bs *BlobService) StartUploadSession(uuid uuid.UUID) error {
	bs.uploadMx.Lock()
	defer bs.uploadMx.Unlock()

	tempFile := filepath.Join(bs.BlobDir, "uploads", uuid.String())
	if err := os.MkdirAll(filepath.Dir(tempFile), 0755); err != nil {
		return err
	}
	file, err := os.Create(tempFile)
	if err != nil {
		return err
	}

	bs.UploadSessions[uuid] = file
	return nil
}

func (bs *BlobService) UploadChunk(sessionID uuid.UUID, r io.ReadCloser) (int64, error) {
	bs.uploadMx.Lock()
	defer bs.uploadMx.Unlock()

	file, ok := bs.UploadSessions[sessionID]
	if !ok {
		return 0, ErrUploadSessionNotFound
	}

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	offset := info.Size()
	n, err := io.Copy(file, r)
	if err != nil {
		return 0, err
	}

	return offset + n - 1, nil
}

func (bs *BlobService) FinishUploadSession(sessionID uuid.UUID, digest string) error {
	bs.uploadMx.Lock()
	tempFile, ok := bs.UploadSessions[sessionID]
	if !ok {
		bs.uploadMx.Unlock()
		return ErrUploadSessionNotFound
	}
	delete(bs.UploadSessions, sessionID)
	bs.uploadMx.Unlock()
	filePath := tempFile.Name()
	defer tempFile.Close()

	const prefix = "sha256:"
	if strings.HasPrefix(digest, prefix) {
		digest = digest[len(prefix):]
	}
	expectedHash := strings.ToLower(digest[len(prefix):])

	hasher := sha256.New()
	if _, err := io.Copy(hasher, tempFile); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}
	computedHash := fmt.Sprintf("%x", hasher.Sum(nil))

	if computedHash != expectedHash {
		_ = os.Remove(tempFile.Name())
		return ErrInvalidHash
	}

	finalPath := filepath.Join(bs.BlobDir, computedHash)
	if err := os.Rename(filePath, finalPath); err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

func (bs *BlobService) GetBlob(hash string) ([]byte, error) {
	blobPath := filepath.Join(bs.BlobDir, hash)
	file, err := os.Open(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBlobNotFound
		}
		return nil, err
	}
	defer file.Close()

	blobBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return blobBytes, nil
}

func (bs *BlobService) GetBlobFile(hash string) (*os.File, error) {
	blobPath := filepath.Join(bs.BlobDir, hash)
	return os.Open(blobPath)
}

func (bs *BlobService) StreamBlob(w io.Writer, reference string) (int64, error) {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	blobFile := filepath.Join(bs.BlobDir, reference)
	file, err := os.Open(blobFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrBlobNotFound
		}
		return 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrBlobNotFound
		}
		return 0, err
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		return 0, err
	}

	return stat.Size(), nil
}

func (bs *BlobService) CreateManifest(repoName string, reference string, data []byte) (string, error) {
	manifestPath := filepath.Join(bs.ManifestDir, repoName)
	if err := os.MkdirAll(manifestPath, 0755); err != nil {
		return "", err
	}

	manifestFile := filepath.Join(manifestPath, reference)
	err := os.WriteFile(manifestFile, data, 0644)
	if err != nil {
		return "", err
	}

	file, err := os.Open(manifestFile)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	manifestHashPath := filepath.Join(manifestPath, hash)
	err = os.WriteFile(manifestHashPath, data, 0644)
	if err != nil {
		return "", err
	}

	return hash, nil
}

func (bs *BlobService) GetManifest(repoName string, reference string) ([]byte, error) {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	manifestPath := filepath.Join(bs.ManifestDir, repoName)
	manifestFile := filepath.Join(manifestPath, reference)

	data, err := os.ReadFile(manifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrManifestNotFound
		}
		return nil, err
	}

	return data, nil
}

func (bs *BlobService) GetManifestLength(repoName string, reference string) (int64, error) {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	manifestPath := filepath.Join(bs.ManifestDir, repoName)
	manifestFile := filepath.Join(manifestPath, reference)

	file, err := os.Open(manifestFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrManifestNotFound
		}
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return stat.Size(), nil
}

func GetHash(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func GetLength(data []byte) int {
	return len(data)
}

func (bs *BlobService) GetBlobLength(repoName string, reference string) (int64, error) {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	blobPath := filepath.Join(bs.BlobDir, repoName)
	blobFile := filepath.Join(blobPath, reference)
	file, err := os.Open(blobFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ErrBlobNotFound
		}
		return 0, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	return stat.Size(), nil
}

func (bs *BlobService) GetBlobHash(repoName string, reference string) (string, error) {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	blobPath := filepath.Join(bs.BlobDir, repoName)
	blobFile := filepath.Join(blobPath, reference)
	file, err := os.Open(blobFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrBlobNotFound
		}
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return hash, nil
}
