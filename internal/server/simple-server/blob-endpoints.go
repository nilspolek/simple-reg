package simpleserver

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nilspolek/simple/internal/server"
)

func handleStartUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	uploadID := uuid.New()

	if err := blobService.StartUpload(uploadID); err != nil {
		server.WriteErrors(w, server.ERROR_BLOB_UNKNOWN)
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

func handlePatchBlob(w http.ResponseWriter, r *http.Request) {
	sessionID := uuid.MustParse(mux.Vars(r)["id"])

	end, err := blobService.WriteChunk(sessionID, r.Body)
	defer r.Body.Close()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	uploadID := uuid.MustParse(vars["id"])
	repo := vars["name"]

	digest := r.URL.Query().Get("digest")
	if digest == "" {
		http.Error(w, "digest required", http.StatusBadRequest)
		return
	}

	blobService.FinalizeUpload(uploadID, digest)

	location := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	w.Header().Set("Location", location)
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", "0")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(http.StatusCreated)
}

func handleGetBlob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"]

	blob, err := blobService.StreamBlob(digest)
	defer blob.Close()
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}

	info, err := blob.Stat()
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
	if _, err := io.Copy(w, blob); err != nil {
		log.Println("error while streaming blob:", err)
	}
}

func handleBlobHeaders(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	digest := vars["digest"] // e.g., "sha256:abc123...	if !sha256Regex.MatchString(digest) {

	blob, err := blobService.StreamBlob(digest)
	if err != nil {
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}
	defer blob.Close()

	info, err := blob.Stat()
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
