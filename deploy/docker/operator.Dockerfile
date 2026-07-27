FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.work ./
COPY services/control-plane/go.mod ./services/control-plane/go.mod
COPY services/control-plane/go.sum ./services/control-plane/go.sum
COPY services/operator/go.mod ./services/operator/go.mod
COPY services/operator/go.sum ./services/operator/go.sum
COPY services/operator ./services/operator

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/buildplane-operator \
    ./services/operator/cmd/buildplane-operator

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/buildplane-operator /buildplane-operator

USER 65532:65532
EXPOSE 8080 8081

ENTRYPOINT ["/buildplane-operator"]

