FROM golang:1.19-alpine AS builder

COPY . .
RUN CGO_ENABLED=0 GOARCH=amd64 \
  go build -mod=vendor -ldflags "-s -w" -o build/mainfluxlabs-mfxkit cmd/mfxkit/main.go \
  && mv build/mainfluxlabs-mfxkit /exe

FROM scratch
COPY --from=builder /exe /
ENTRYPOINT ["/exe"]
