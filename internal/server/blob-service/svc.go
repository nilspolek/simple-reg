package blobservice

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var (
	BlobDir           = "./data/blobs"
	ErrUploadNotFound = errors.New("upload not found")
	ErrDigestMismatch = errors.New("digest mismatch")
)

type BlobService struct {
	UploadSessions map[uuid.UUID]*os.File
	sync.Mutex
}

func New() *BlobService {
	return &BlobService{
		UploadSessions: map[uuid.UUID]*os.File{},
		Mutex:          sync.Mutex{},
	}
}

func (bs *BlobService) StartUpload(uploadID uuid.UUID) error {
	bs.Mutex.Lock()
	defer bs.Mutex.Unlock()
	filePath := filepath.Join(BlobDir, "uploads", uploadID.String())
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	bs.UploadSessions[uploadID] = file
	return nil
}

func (bs *BlobService) WriteChunk(uploadID uuid.UUID, r io.ReadCloser) (int64, error) {
	bs.Mutex.Lock()
	defer bs.Mutex.Unlock()
	file, ok := bs.UploadSessions[uploadID]
	if !ok {
		return 0, ErrUploadNotFound
	}

	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(file, r)
	if err != nil {
		return 0, err
	}

	return info.Size() + n - 1, nil
}

func (bs *BlobService) FinalizeUpload(uploadID uuid.UUID, digest string) error {
	bs.Mutex.Lock()
	defer bs.Mutex.Unlock()
	file, ok := bs.UploadSessions[uploadID]
	if !ok {
		return ErrUploadNotFound
	}

	if err := file.Close(); err != nil {
		return err
	}
	delete(bs.UploadSessions, uploadID)

	filePath := file.Name()
	file.Close()

	digest = ensureNoShaPrefix(digest)
	hasher := sha256.New()
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != digest {
		return ErrDigestMismatch
	}

	finalPath := filepath.Join(BlobDir, digest)
	if err := os.Rename(filePath, finalPath); err != nil {
		return err
	}

	return nil
}

func ensureNoShaPrefix(digest string) string {
	if strings.HasPrefix(digest, "sha256:") {
		return digest[len("sha256:"):]
	}
	return digest
}

func (bs *BlobService) StreamBlob(digest string) (*os.File, error) {
	digest = ensureNoShaPrefix(digest)
	filePath := filepath.Join(BlobDir, digest)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return file, nil
}
