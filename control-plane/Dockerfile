FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/control-plane ./cmd/api
# The document directory has to exist in the image with the right owner. Docker
# seeds a fresh named volume from the image path, ownership included, and the
# final stage has no shell to mkdir or chown with.
RUN mkdir -p /out/documents

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/control-plane /control-plane
# 65532 is `nonroot` in the distroless images. Numeric so the COPY does not
# depend on the base image's /etc/passwd.
COPY --from=build --chown=65532:65532 /out/documents /var/lib/chiotron/documents
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/control-plane"]
