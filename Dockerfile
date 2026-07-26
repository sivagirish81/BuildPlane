FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/scheduler ./cmd/scheduler
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/autoscaler ./cmd/autoscaler
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/runner ./cmd/runner
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/bpctl ./cmd/bpctl
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/migrate ./cmd/migrate

FROM alpine:3.22 AS service
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ /usr/local/bin/
COPY migrations ./migrations
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/api"]

FROM alpine:3.22 AS runner
RUN apk add --no-cache ca-certificates git go
WORKDIR /app
COPY --from=build /out/runner /usr/local/bin/runner
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/runner"]
