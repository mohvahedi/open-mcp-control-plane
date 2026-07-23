# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/controlplane ./cmd/controlplane \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/mcpctl ./cmd/mcpctl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/controlplane /controlplane
COPY --from=build /out/mcpctl /mcpctl
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/controlplane"]
