# sounding-server 多阶段构建
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/sounding-server ./cmd/server

FROM alpine:3.20
COPY --from=build /out/sounding-server /usr/local/bin/sounding-server
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["sounding-server"]
CMD ["-addr", ":8080", "-db", "/data/sounding.db", "-static", "/admin"]
