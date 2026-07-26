FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.work ./
COPY services/control-plane/go.mod ./services/control-plane/go.mod
COPY services/control-plane ./services/control-plane

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/buildplane-control-plane \
    ./services/control-plane/cmd/buildplane-control-plane

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/buildplane-control-plane /buildplane-control-plane

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/buildplane-control-plane"]
