FROM golang:1.15.6-alpine3.12 as dev

RUN apk add --no-cache gcc libc-dev

FROM dev as build

ARG VERSION
ARG COMMIT

WORKDIR /workspace
ADD . /workspace/
RUN go build -ldflags "-X main.version=${VERSION:-dev} -X main.commit=${COMMIT:-none}" ./cmd/gtokenserver

FROM alpine:3.11.2

WORKDIR /workspace
COPY LICENSE /
COPY --from=build /workspace/gtokenserver /

ENTRYPOINT ["/gtokenserver", "--host", "0.0.0.0"]
