package simpleserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	blobservice "github.com/nilspolek/simple/internal/blob-service"
)

type routes struct {
	blobService *blobservice.BlobService
}

func NewRoutes(blobService *blobservice.BlobService) routes {
	return routes{
		blobService: blobService,
	}
}

func (_ routes) defaultEndpoint(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Welcome to Simple Server v%d!", VERSION)
}

func GetScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func (routes routes) handleStartUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	uploadID := uuid.New()
	err := routes.blobService.StartUploadSession(uploadID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to start upload session: %v", err), http.StatusInternalServerError)
		return
	}

	location := fmt.Sprintf("%s://%s/v2/%s/blobs/uploads/%s", GetScheme(r), r.Host, repo, uploadID.String())
	w.Header().Set("Location", location)
	w.Header().Set("Docker-Upload-UUID", uploadID.String())
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Range", "0-0")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Docker-Distribution-Api-Version", fmt.Sprintf("registry/%d.%d", VERSION, PATCH_VERSION))
	w.WriteHeader(http.StatusAccepted)
}

func (routes routes) handlePatchBlob(w http.ResponseWriter, r *http.Request) {
	sessionID, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	n, err := routes.blobService.UploadChunk(sessionID, r.Body)
	defer r.Body.Close()

	// Docker-konforme Header setzen
	w.Header().Set("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", mux.Vars(r)["name"], sessionID))
	w.Header().Set("Range", fmt.Sprintf("0-%d", n))
	w.Header().Set("Docker-Upload-UUID", sessionID.String())
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusAccepted)
}

func (routes routes) handleFinalizeUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	sessionID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid UUID: %v", err), http.StatusBadRequest)
		return
	}

	repo := vars["name"]
	digest := r.URL.Query().Get("digest")
	if digest == "" {
		http.Error(w, "digest required", http.StatusBadRequest)
		return
	}

	log.Printf("Finalize upload called with digest: %q", digest)

	err = routes.blobService.FinishUploadSession(sessionID, digest)
	if err != nil {
		if errors.Is(err, blobservice.ErrUploadSessionNotFound) {
			http.Error(w, "upload session not found", http.StatusNotFound)
		} else if errors.Is(err, blobservice.ErrInvalidHash) {
			http.Error(w, "digest mismatch", http.StatusBadRequest)
		} else {
			http.Error(w, fmt.Sprintf("internal server error %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Successful response
	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusCreated)
}

func (routes routes) handlePutManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read manifest", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	hash, err := routes.blobService.CreateManifest(repo, ref, data)

	if err != nil {
		http.Error(w, "failed to create manifest", http.StatusInternalServerError)
		return
	}

	digest := fmt.Sprintf("sha256:%x", hash)
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
func (routes routes) handleGetManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	data, err := routes.blobService.GetManifest(repo, ref)
	length := blobservice.GetLength(data)
	hash := blobservice.GetHash(data)
	if err != nil {
		http.Error(w, fmt.Sprintf("manifest [%s] not found", ref), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	w.Header().Set("Docker-Content-Digest", fmt.Sprintf("sha256:%x", hash))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (routes routes) handleGetBlob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"]

	file, err := routes.blobService.GetBlobFile(digest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	s, _ := file.Stat()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", s.Size()))
	w.WriteHeader(http.StatusOK)

	_, err = routes.blobService.StreamBlob(w, digest)
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}
}

func (routes routes) handleBlobHeaders(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"]

	blob, err := routes.blobService.GetBlob(digest)
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}

	length := blobservice.GetLength(blob)
	hash := blobservice.GetHash(blob)

	w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Docker-Content-Digest", fmt.Sprintf("sha256:%x", hash))
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusOK)
}
