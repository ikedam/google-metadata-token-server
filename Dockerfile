FROM golang:1.15.6-alpine3.12 as dev

RUN apk add --no-cache gcc libc-dev
