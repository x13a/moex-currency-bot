FROM golang:alpine

WORKDIR /build/
COPY ./src/go.mod ./src/go.sum ./
COPY ./src/*.go ./

RUN go mod download
RUN go build -o ./main

FROM alpine

COPY --from=0 /build/main /
LABEL com.centurylinklabs.watchtower.enable="true"

USER nobody:nogroup
STOPSIGNAL SIGINT

# ENV HEALTHCHECK_ENABLE 1
# HEALTHCHECK CMD healthy http://127.0.0.1:8000/ping || exit 1

ENTRYPOINT ["/main"]
EXPOSE 8000
