# Simple Registry

Simple Registry is a lightweight implementation of a Docker Registry API. It provides endpoints for managing blobs, manifests, and tags in a Docker-compatible registry. This project is designed to be simple and extensible, making it suitable for small-scale deployments or as a starting point for custom registry implementations.

## Features

- **Blob Management**: Upload, patch, finalize, and retrieve blobs.
- **Manifest Management**: Create, retrieve, and delete manifests.
- **Tag Management**: List tags for repositories.
- **Docker-Compatible API**: Implements Docker Registry API endpoints.
- **Logging**: Integrated logging using `zerolog`.
- **Thread-Safe Operations**: Ensures thread safety for blob and manifest operations.

## Directory Structure

- `internal/server/blob-service/`: Contains the implementation for blob-related operations.
- `internal/server/manifest-service/`: Contains the implementation for manifest-related operations.
- `internal/server/simple-server/`: Contains the HTTP handlers for the registry endpoints.
- `internal/server/`: Contains shared utilities and the main server implementation.

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/simple-reg.git
   cd simple-reg
   ```
2. Build the project:
   ```bash
   go build ./...
   ```
3. Run the server:
   ```bash
   ./simple-reg
   ```

## API Endpoints
### Blob Endpoints
- **Start Upload**: `POST /v2/{name}/blobs/uploads/`
- **Patch Blob**: `PATCH /v2/{name}/blobs/uploads/{id}`
- **Finalize Upload**: `PUT /v2/{name}/blobs/uploads/{id}?digest=sha256:<digest>`
- **Get Blob**: `GET /v2/{name}/blobs/{digest}`
- **Blob Headers**: `HEAD /v2/{name}/blobs/{digest}`

### Manifest Endpoints
- **Create Manifest**: `PUT /v2/{name}/manifests/{reference}`
- **Get Manifest**: `GET /v2/{name}/manifests/{reference}`
- **Delete Manifest**: `DELETE /v2/{name}/manifests/{reference}`

### Tag Endpoints
- **List Tags**: `GET /v2/{name}/tags/list`
- **List all Tags**: `GET /v2/tags/list`

## Logging
The logging system is integrated using `zerolog`. It provides structured logging capabilities and can be configured to output logs in JSON format for easy parsing and analysis.
