FROM golang:1.23.12 AS build
WORKDIR /src
COPY . .
RUN go test ./... && go vet ./... && go build -o /out/edgeconfig ./cmd/edgeconfig

FROM debian:bookworm-slim
COPY --from=build /out/edgeconfig /usr/local/bin/edgeconfig
ENTRYPOINT ["/usr/local/bin/edgeconfig"]
