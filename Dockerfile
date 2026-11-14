FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/cairn ./cmd/cairn \
 && mkdir -p /out/data

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/cairn /usr/local/bin/cairn
# Named volumes copy this ownership on first mount, so /data is writable by nonroot.
COPY --from=build --chown=65532:65532 /out/data /data
ENV CAIRN_DB_PATH=/data/cairn.db CAIRN_ADDR=:8080 CAIRN_REVIEW_ADDR=:8081
USER nonroot:nonroot
VOLUME /data
EXPOSE 8080 8081
ENTRYPOINT ["/usr/local/bin/cairn"]
