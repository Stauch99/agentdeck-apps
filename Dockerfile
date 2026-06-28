# ---- build: static binary, no cgo, frontend embedded ----
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /openmusic .

# ---- run: scratch + CA certs (TLS to api.kie.ai) ----
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /openmusic /openmusic
EXPOSE 8080
ENV OPENMUSIC_ADDR=:8080 OPENMUSIC_DATA_DIR=/data
ENTRYPOINT ["/openmusic"]
