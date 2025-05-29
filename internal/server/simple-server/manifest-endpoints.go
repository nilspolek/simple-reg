package simpleserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	blobservice "github.com/nilspolek/simple-reg/internal/server/blob-service"
	manifestservice "github.com/nilspolek/simple-reg/internal/server/manifest-service"
)

var (
	blobService     = blobservice.New()
	manifestService = manifestservice.New()
)

func GetScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func handlePutManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	hash, err := manifestService.CreateManifest(data, repo, ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set Docker Registry compliant headers
	w.Header().Set("Location", fmt.Sprintf("/v2/%s/manifests/%s", repo, ref))
	w.Header().Set("Content-Length", "0") // No response body
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	w.Header().Set("Docker-Content-Digest", hash)
	w.WriteHeader(http.StatusCreated)
}

func handleGetManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	manifest, hash, err := manifestService.GetManifest(repo, ref)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(manifest)))
	w.Header().Set("Docker-Content-Digest", hash)
	w.WriteHeader(http.StatusOK)
	w.Write(manifest)
}

type RepoTag struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func handleGetAllTags(w http.ResponseWriter, r *http.Request) {
	allTags := manifestService.GetAllTags()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	repoTags := make([]RepoTag, 0)
	for repo, tag := range allTags {
		repoTags = append(repoTags, RepoTag{
			Name: repo,
			Tags: tag,
		})
	}

	if err := json.NewEncoder(w).Encode(repoTags); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleGetTags(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]

	tags := manifestService.GetTags(repo)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	tagDTO := RepoTag{
		Name: repo,
		Tags: tags,
	}

	if err := json.NewEncoder(w).Encode(tagDTO); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleDeleteManifest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repo := vars["name"]
	ref := vars["reference"]

	if err := manifestService.DeleteManifest(repo, ref); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
