# sounding-server 多阶段构建（内置 ink 监控面板）
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/sounding-server ./cmd/server

# 可选：构建时拉取 ink 监控面板（ARG 控制，离线构建可跳过）
FROM alpine:3.20 AS ink
ARG INSTALL_INK=1
RUN if [ "$INSTALL_INK" = "1" ]; then \
      apk add --no-cache curl unzip && \
      TAG=$(curl -fsSL https://api.github.com/repos/jacob-bytes/komari-theme-ink/releases/latest \
        | grep -o '"tag_name": *"[^"]*"' | head -1 | cut -d'"' -f4) && \
      mkdir -p /admin && \
      curl -fsSL "https://github.com/jacob-bytes/komari-theme-ink/releases/download/$TAG/ink-build-${TAG#v}.zip" -o /tmp/ink.zip && \
      unzip -q /tmp/ink.zip -d /tmp/ink && cp -r /tmp/ink/dist/. /admin/ && rm -rf /tmp/ink /tmp/ink.zip; \
    else mkdir -p /admin; fi

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/sounding-server /usr/local/bin/sounding-server
COPY --from=ink /admin /admin
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["sounding-server"]
CMD ["-addr", ":8080", "-db", "/data/sounding.db", "-static", "/admin"]
