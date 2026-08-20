# Build the three binaries in one pass; they share the same module cache.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Dependencies first, so a source-only change does not refetch modules.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api    ./cmd/api  && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed   ./cmd/seed

# Static binaries need almost nothing at runtime: certificates for outbound TLS
# and a timezone database, since rental dates are calendar dates.
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 10001 rental
COPY --from=build /out/api    /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /out/seed   /usr/local/bin/seed

USER rental
ENV PORT=8080
EXPOSE 8080
# The worker and seed binaries are selected by overriding the command.
CMD ["/usr/local/bin/api"]
