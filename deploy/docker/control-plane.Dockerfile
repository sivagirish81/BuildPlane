FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.work ./
COPY services/control-plane/go.mod ./services/control-plane/go.mod
COPY services/control-plane/go.sum ./services/control-plane/go.sum
COPY services/operator/go.mod ./services/operator/go.mod
COPY services/operator/go.sum ./services/operator/go.sum
COPY services/control-plane ./services/control-plane

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/buildplane-control-plane \
    ./services/control-plane/cmd/buildplane-control-plane
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/buildplane-scheduler \
    ./services/control-plane/cmd/buildplane-scheduler
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/buildplane-worker \
    ./services/control-plane/cmd/buildplane-worker

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/buildplane-control-plane /buildplane-control-plane
COPY --from=build /out/buildplane-scheduler /buildplane-scheduler
COPY --from=build /out/buildplane-worker /buildplane-worker
COPY migrations /migrations

USER 65532:65532
EXPOSE 8080
ENV BUILDPLANE_MIGRATIONS_DIR=/migrations

ENTRYPOINT ["/buildplane-control-plane"]
