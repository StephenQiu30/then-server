FROM golang:1.26.5@sha256:705e964a93a2fd2e75c7d59bb7d781b57e30f12293ffde5175c69229e18fb678 AS build
WORKDIR /src
ENV GOTOOLCHAIN=local CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY main.go openapi.yaml ./
COPY internal ./internal
RUN go build -mod=readonly -trimpath -buildvcs=false -ldflags="-s -w" -o /out/then-backend . \
    && go list -m -json all > /out/modules.json

FROM scratch
COPY --from=build /out/then-backend /then-backend
COPY --from=build /out/modules.json /usr/share/then/modules.json
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 65532:65532
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/then-backend"]
