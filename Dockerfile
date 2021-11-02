FROM golang:1.17-alpine AS build

WORKDIR /src
COPY . .
RUN go build -o /out/terrastack ./cmd/terrastack

FROM alpine:latest AS bin
COPY --from=build /out/terrastack /bin/terrastack
