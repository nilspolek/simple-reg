# Simple Registry

Simple Registry is a lightweight implementation of a Docker Registry API. It provides endpoints for managing blobs, manifests, and tags in a Docker-compatible registry. This project is designed to be simple and extensible, making it suitable for small-scale deployments or as a starting point for custom registry implementations.

## Features

* **Blob Management**: Upload, patch, finalize, and retrieve blobs.
* **Manifest Management**: Create, retrieve, and delete manifests.
* **Tag Management**: List tags for repositories.
* **Docker-Compatible API**: Implements Docker Registry API endpoints.
* **Logging**: Integrated logging using `zerolog`.
* **Thread-Safe Operations**: Ensures thread safety for blob and manifest operations.
* **Configurable Port and Verbosity**: Use `-port` to set the server port and `-verbose` for detailed logs.

## Directory Structure

* `internal/server/blob-service/`: Contains the implementation for blob-related operations.
* `internal/server/manifest-service/`: Contains the implementation for manifest-related operations.
* `internal/server/simple-server/`: Contains the HTTP handlers for the registry endpoints.
* `internal/server/`: Contains shared utilities and the main server implementation.

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/nilspolek/simple-reg.git
   cd simple-reg
   ```

2. Build the project:

   ```bash
   go build -o bin/simple-reg cli/simple-reg/main.go
   ```

3. Run the server (default port: 5000):

   ```bash
   ./bin/simple-reg -port 5000 -verbose
   ```

   * Use `-port` to set a custom port (default is `5000`)
   * Use `-verbose` to enable verbose (debug-level) logging

### Install with Go

To install the application using Go, run the following command:

```bash
go install github.com/nilspolek/simple-reg/cli/simple-reg@latest
```

## API Endpoints

### Blob Endpoints

* **Start Upload**: `POST /v2/{name}/blobs/uploads/`
* **Patch Blob**: `PATCH /v2/{name}/blobs/uploads/{id}`
* **Finalize Upload**: `PUT /v2/{name}/blobs/uploads/{id}?digest=sha256:<digest>`
* **Get Blob**: `GET /v2/{name}/blobs/{digest}`
* **Blob Headers**: `HEAD /v2/{name}/blobs/{digest}`

### Manifest Endpoints

* **Create Manifest**: `PUT /v2/{name}/manifests/{reference}`
* **Get Manifest**: `GET /v2/{name}/manifests/{reference}`
* **Delete Manifest**: `DELETE /v2/{name}/manifests/{reference}`

### Tag Endpoints

* **List Tags**: `GET /v2/{name}/tags/list`
* **List All Tags**: `GET /v2/tags/list`

## Logging

The logging system is integrated using `zerolog`. It provides structured logging capabilities and can be configured to output logs in JSON format for easy parsing and analysis.

* Use `-verbose` to enable debug-level logs.
* Default log output is in structured JSON.

## Docker Integration

You can use the Docker CLI to interact with your Simple Registry server.

### Push an Image

1. Tag the image:

   ```bash
   docker tag alpine localhost:5000/myrepo/alpine:latest
   ```

2. Push the image:

   ```bash
   docker push localhost:5000/myrepo/alpine:latest
   ```

   Make sure the server is running and accessible on the specified port.

### Pull an Image

1. Pull the image:

   ```bash
   docker pull localhost:5000/myrepo/alpine:latest
   ```

> 💡 You may need to configure your Docker daemon to allow insecure registries for `localhost:5000` by editing your Docker daemon config:
>
> ```json
> {
>   "insecure-registries": ["localhost:5000"]
> }
> ```

Then restart Docker.
