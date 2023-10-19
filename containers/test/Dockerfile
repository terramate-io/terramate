# Copyright 2023 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.20-alpine3.16

RUN apk add --no-cache git gcc g++

# Needed for go test -race
ENV CGO_ENABLED=1

WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    go test -race -count=1 -v ./cmd/terramate/e2etests
