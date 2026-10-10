FROM node:22-bookworm-slim AS web
WORKDIR /src
COPY web/package*.json ./web/
RUN npm --prefix web ci
COPY web ./web
RUN npm --prefix web run build

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/web/dist/app ./internal/web/dist/app
RUN CGO_ENABLED=0 go build -trimpath -o /inftask ./cmd/inftask

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && apt-get clean && useradd --uid 10001 --create-home inftask && mkdir /data && chown inftask:inftask /data
COPY --from=build /inftask /usr/local/bin/inftask
USER 10001:10001
VOLUME /data
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/inftask"]
CMD ["-addr", "0.0.0.0:8787", "-data", "/data/inftask.db"]
