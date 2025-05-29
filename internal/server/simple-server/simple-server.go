package simpleserver

import (
	"fmt"
	"net/http"
	"os"

	"github.com/nilspolek/simple/internal/server"
)

const (
	VERSION       = 2
	PATCH_VERSION = 0
)

var (
	BlobDir     = "./data/blobs"
	ManifestDir = "./data/manifests"
)

func New(blobdir ...string) *server.Server {
	if len(blobdir) > 0 {
		BlobDir = blobdir[0]
	}
	svr := server.NewServer()
	setupRoutes(svr)

	// create blobdir if it doesn't exist
	if _, err := os.Stat(BlobDir); os.IsNotExist(err) {
		if err := os.MkdirAll(BlobDir, 0755); err != nil {
			panic(err)
		}
	}

	if _, err := os.Stat(ManifestDir); os.IsNotExist(err) {
		if err := os.MkdirAll(ManifestDir, 0755); err != nil {
			panic(err)
		}
	}
	return svr
}

func setupRoutes(svr *server.Server) {

	svr.Router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svr.GetLogger().Debug().Msg(fmt.Sprintf("method not found %s [%s]", r.Method, r.URL.Path))
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	svr.Router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svr.GetLogger().Debug().Msg(fmt.Sprintf("method not allowed %s [%s]", r.Method, r.URL.Path))
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	prefix := fmt.Sprintf("/v%d", VERSION)
	svr.WithHandlerFunc(prefix+"/", handleGetAllTags, "GET")
	svr.WithHandlerFunc(prefix+"/{name:.+}/blobs/uploads/", handleStartUpload, http.MethodPost)
	svr.WithHandlerFunc(prefix+"/{name:.+}/blobs/uploads/{id}", handleFinalizeUpload, http.MethodPut)
	svr.WithHandlerFunc(prefix+"/{name:.+}/blobs/uploads/{id}", handlePatchBlob, http.MethodPatch)
	svr.WithHandlerFunc(prefix+"/{name:.+}/blobs/{digest}", handleBlobHeaders, http.MethodGet, http.MethodHead)
	svr.WithHandlerFunc(prefix+"/{name:.+}/blobs/{digest}", handleGetBlob, http.MethodGet)

	// manifest
	svr.WithHandlerFunc(prefix+"/{name:.+}/manifests/{reference:.+}", handleGetManifest, http.MethodGet, http.MethodHead)
	svr.WithHandlerFunc(prefix+"/{name:.+}/manifests/{reference:.+}", handlePutManifest, http.MethodPut)
	svr.WithHandlerFunc(prefix+"/{name:.+}/manifests/{reference:.+}", handleDeleteManifest, http.MethodDelete)

	// tag
	svr.WithHandlerFunc(prefix+"/tags/list", handleGetAllTags, http.MethodGet)
	svr.WithHandlerFunc(prefix+"/{name:.+}/tags/list", handleGetTags, http.MethodGet)
}
