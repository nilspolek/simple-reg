package blobservice

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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
	currentUploads map[uuid.UUID]*os.File
	uploadMx       sync.Mutex
}

func New() *BlobService {
	return &BlobService{
		BlobDir:        DefaultBlobDir,
		ManifestDir:    DefaultManifestDir,
		currentUploads: make(map[uuid.UUID]*os.File),
		uploadMx:       sync.Mutex{},
	}
}

func (bs *BlobService) StartUploadSession(repoName string) (uuid.UUID, error) {
	bs.uploadMx.Lock()
	defer bs.uploadMx.Unlock()

	sessionID := uuid.New()
	filePath := filepath.Join(bs.BlobDir, "uploads", sessionID.String())
	file, err := os.Create(filePath)
	if err != nil {
		return uuid.Nil, err
	}
	bs.currentUploads[sessionID] = file

	return sessionID, nil
}

func (bs *BlobService) UploadChunk(sessionID uuid.UUID, chunk []byte) (error, int64) {
	bs.uploadMx.Lock()
	defer bs.uploadMx.Unlock()

	file, ok := bs.currentUploads[sessionID]
	if !ok {
		return ErrUploadSessionNotFound, 0
	}

	info, err := file.Stat()
	if err != nil {
		return err, 0
	}

	_, err = file.Write(chunk)
	if err != nil {
		return err, 0
	}

	return nil, info.Size() + int64(len(chunk))
}

func (bs *BlobService) FinishUploadSession(sessionID uuid.UUID, digits string) error {
	bs.uploadMx.Lock()
	defer bs.uploadMx.Unlock()

	file, ok := bs.currentUploads[sessionID]
	if !ok {
		return ErrUploadSessionNotFound
	}

	hasher := sha256.New()
	_, err := io.Copy(hasher, file)
	if err != nil {
		return err
	}
	err = file.Close()
	if err != nil {
		return err
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	if hash != digits {
		return ErrInvalidHash
	}
	delete(bs.currentUploads, sessionID)
	return nil
}

func (bs *BlobService) GetBlob(w io.Writer, repoName string, reference string) error {
	if strings.HasPrefix(reference, "sha256:") {
		reference = strings.TrimPrefix(reference, "sha256:")
	}

	blobPath := filepath.Join(bs.BlobDir, repoName)
	blobFile := filepath.Join(blobPath, reference)
	file, err := os.Open(blobFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrBlobNotFound
		}
		return err
	}
	defer file.Close()

	_, err = io.Copy(w, file)
	if err != nil {
		return err
	}

	return nil
}

func (bs *BlobService) CreateManifest(repoName string, reference string, data []byte) error {
	manifestPath := filepath.Join(bs.ManifestDir, repoName)
	if err := os.MkdirAll(manifestPath, 0755); err != nil {
		return err
	}

	manifestFile := filepath.Join(manifestPath, reference)
	file, err := os.Create(manifestFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return err
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	manifestHashPath := filepath.Join(manifestPath, hash)
	err = os.WriteFile(manifestHashPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (bs *BlobService) StreamManifest(repoName string, reference string) ([]byte, error) {
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
