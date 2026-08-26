# Container Build Notes

The service is built from the repository root with Go 1.23.12. The container build is intentionally dependency-free and produces the `edgeconfig` binary from `./cmd/edgeconfig`.

Validation uses `go test ./...`, `go vet ./...`, and `go build ./...` inside the pinned Linux container image.
