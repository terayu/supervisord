FROM golang:alpine AS builder

RUN apk add --no-cache --update git gcc rust

COPY . /src
WORKDIR /src

# -tags release embeds webgui/ into the binary. Without it the image serves the
# web GUI from ./webgui relative to the working directory, which does not exist
# in a scratch image, and every GUI route answers 404.
#
# CGO_ENABLED=0 already produces a static binary, so the ldflags this used to
# carry were both unnecessary and self-contradictory: -linkmode external asks
# for the external (cgo) linker that CGO_ENABLED=0 disables, and the build
# failed with "-linkmode requires external (cgo) linking, but cgo is not
# enabled".
RUN CGO_ENABLED=0 go build -tags release -a -o /usr/local/bin/supervisord github.com/ochinchina/supervisord

FROM scratch

COPY --from=builder /usr/local/bin/supervisord /usr/local/bin/supervisord

ENTRYPOINT ["/usr/local/bin/supervisord"]
