# Build stage
FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -o /out/mcp-pipedrive ./cmd/mcp-pipedrive

# Runtime stage
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=build /out/mcp-pipedrive /app/mcp-pipedrive

ENV PIPEDRIVE_CACHE_PATH=/app/.cache/pipedrive-mcp.bbolt

ENTRYPOINT ["/app/mcp-pipedrive"]
