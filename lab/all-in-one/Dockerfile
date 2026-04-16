FROM golang:1.24 AS build

WORKDIR /src
COPY . .
RUN go build -o /out/alfred ./cmd/alfred

FROM rancher/k3s:v1.30.3-k3s1

COPY --from=build /out/alfred /usr/local/bin/alfred
COPY lab/all-in-one/entrypoint.sh /entrypoint.sh
COPY lab/all-in-one/manifests /lab/manifests
COPY lab/all-in-one/scripts /lab/scripts

RUN chmod +x /entrypoint.sh /lab/scripts/*.sh

ENTRYPOINT ["/entrypoint.sh"]
