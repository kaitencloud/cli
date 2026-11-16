# Built by GoReleaser from the release binaries; see dockers_v2 in
# .goreleaser.yaml. The build context holds one binary per platform under
# <os>/<arch>/, and the static distroless base carries CA certificates and a
# non-root user and nothing else.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/kaiten /usr/local/bin/kaiten
COPY LICENSE NOTICE /usr/share/doc/kaiten/

ENTRYPOINT ["/usr/local/bin/kaiten"]
