package simpleserver

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nilspolek/simple/internal/server"
)

var uploadSessions = struct {
	sync.Mutex
	sessions map[uuid.UUID]*os.File
}{sessions: make(map[uuid.UUID]*os.File)}

func defaultEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Welcome to Simple Server v%d!", VERSION)
}

func GetScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func handleStartUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	uploadID := uuid.New()

	// Neuen Upload-File-Pfad, nur nach UUID
	filePath := filepath.Join(BlobDir, "uploads", uploadID.String())
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		http.Error(w, "failed to create upload directory", http.StatusInternalServerError)
		return
	}

	file, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "failed to create upload file", http.StatusInternalServerError)
		return
	}

	uploadSessions.Lock()
	uploadSessions.sessions[uploadID] = file
	uploadSessions.Unlock()

	location := fmt.Sprintf("%s://%s/v2/%s/blobs/uploads/%s", GetScheme(r), r.Host, repo, uploadID.String())
	w.Header().Set("Location", location)
	w.Header().Set("Docker-Upload-UUID", uploadID.String())
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Range", "0-0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Docker-Distribution-Api-Version", fmt.Sprintf("registry/%d.%d", VERSION, PATCH_VERSION))
	w.WriteHeader(http.StatusAccepted)
}

func handlePatchBlob(w http.ResponseWriter, r *http.Request) {
	sessionID := uuid.MustParse(mux.Vars(r)["id"])

	uploadSessions.Lock()
	file, ok := uploadSessions.sessions[sessionID]
	log.Print(file.Name())
	uploadSessions.Unlock()
	if !ok {
		server.WriteErrors(w, server.ERROR_BLOB_UNKNOWN)
		return
	}
	defer r.Body.Close()

	// Ermittle aktuellen Offset (Dateiende)
	info, err := file.Stat()
	if err != nil {
		server.WriteErrors(w, server.ERROR_BLOB_UNKNOWN)
		return
	}
	offset := info.Size()

	// Hänge Chunk an Datei an
	n, err := io.Copy(file, r.Body)
	if err != nil {
		server.WriteErrors(w, server.ERROR_BLOB_UNKNOWN)
		return
	}
	end := offset + n - 1

	// Docker-konforme Header setzen
	w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", mux.Vars(r)["name"], sessionID))
	w.Header().Set("Range", fmt.Sprintf("0-%d", end))
	w.Header().Set("Docker-Upload-UUID", sessionID.String())
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusAccepted)
}

func handleFinalizeUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := uuid.MustParse(vars["id"])
	repo := vars["name"]

	digest := r.URL.Query().Get("digest")
	if digest == "" {
		http.Error(w, "digest required", http.StatusBadRequest)
		return
	}
	log.Printf("Finalize upload called with digest: %q", digest)

	uploadSessions.Lock()
	file, ok := uploadSessions.sessions[sessionID]
	if !ok {
		uploadSessions.Unlock()
		http.Error(w, "upload session not found", http.StatusNotFound)
		return
	}
	delete(uploadSessions.sessions, sessionID)
	uploadSessions.Unlock()

	filePath := file.Name()
	file.Close()

	const prefix = "sha256:"
	if !strings.HasPrefix(digest, prefix) {
		http.Error(w, "unsupported digest format", http.StatusBadRequest)
		return
	}

	expected := strings.ToLower(digest[len(prefix):])

	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "failed to open uploaded file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		http.Error(w, "failed to hash uploaded blob", http.StatusInternalServerError)
		return
	}

	computed := fmt.Sprintf("%x", hasher.Sum(nil))
	log.Printf("Expected digest: %q", expected)
	log.Printf("Computed digest: %q", computed)

	if computed != expected {
		log.Printf("Digest mismatch: expected %s but got %s", expected, computed)
		http.Error(w, "digest mismatch", http.StatusBadRequest)
		_ = os.Remove(filePath)
		return
	}

	finalPath := filepath.Join(BlobDir, computed)
	if err := os.Rename(filePath, finalPath); err != nil {
		http.Error(w, "failed to move file", http.StatusInternalServerError)
		return
	}
	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusCreated)
}

type Manifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Config        struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"config"`
	Layers []struct {
		MediaType string `json:"mediaType"`
		Size      int64  `json:"size"`
		Digest    string `json:"digest"`
	} `json:"layers"`
}

func handlePutManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read manifest", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Optional: Validate JSON format
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		http.Error(w, "invalid manifest format", http.StatusBadRequest)
		return
	}

	// Save manifest file
	manifestDir := filepath.Join(ManifestDir, repo)
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		http.Error(w, "failed to create manifest dir", http.StatusInternalServerError)
		return
	}
	manifestPath := filepath.Join(manifestDir, ref)

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		http.Error(w, "failed to save manifest", http.StatusInternalServerError)
		return
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		http.Error(w, "failed to open manifest", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		http.Error(w, "failed to hash manifest", http.StatusInternalServerError)
		return
	}

	digest := fmt.Sprintf("sha256:%x", hasher.Sum(nil))
	os.WriteFile(path.Join(ManifestDir, repo, digest[len("sha256:"):]), data, 0644)

	// Set Docker Registry compliant headers
	w.Header().Set("Location", fmt.Sprintf("/v2/%s/manifests/%s", repo, ref))
	w.Header().Set("Content-Length", "0") // No response body
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	w.Header().Set("Docker-Content-Digest", digest)
	w.WriteHeader(http.StatusCreated)
}

// Retrieve manifest by tag or digest
func handleGetManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	// Extract the digest part
	if strings.HasPrefix(ref, "sha256:") {
		ref = strings.TrimPrefix(ref, "sha256:")
	}

	manifestPath := filepath.Join(ManifestDir, repo, ref)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, fmt.Sprintf("manifest [%s] not found", ref), http.StatusNotFound)
		return
	}

	hasher := sha256.New()
	if _, err := hasher.Write(data); err != nil {
		http.Error(w, "failed to hash manifest", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Docker-Content-Digest", fmt.Sprintf("sha256:%x", hasher.Sum(nil)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func handleGetBlob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"] // z.B. "sha256:abc123..."

	if !strings.HasPrefix(digest, "sha256:") {
		http.Error(w, "invalid digest format", http.StatusBadRequest)
		return
	}

	hashOnly := digest[len("sha256:"):]
	fmt.Println(hashOnly)
	blobPath := filepath.Join(BlobDir, hashOnly)
	file, err := os.Open(blobPath)
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to stat blob file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)

	// Dateiinhalt streamen
	if _, err := io.Copy(w, file); err != nil {
		log.Println("error while streaming blob:", err)
	}
}

var sha256Regex = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

func handleBlobHeaders(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"] // e.g., "sha256:abc123..."
	if !sha256Regex.MatchString(digest) {
		http.Error(w, "invalid digest format", http.StatusBadRequest)
		return
	}

	hashOnly := digest[len("sha256:"):]
	blobPath := filepath.Join(BlobDir, hashOnly)

	// Prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(blobPath), filepath.Clean(BlobDir)) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	file, err := os.Open(blobPath)
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to stat blob file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}
