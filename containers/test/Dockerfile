# Copyright 2023 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.20-alpine3.16 AS base
# Needed for go test -race
RUN apk add --no-cache git gcc g++ make bash
WORKDIR /build
RUN adduser -S tmuser
# Change to non-root user for testing permissions properly
USER tmuser

COPY go.mod .
COPY go.sum .
RUN go mod download

FROM base AS test

ENV CGO_ENABLED=1

RUN --mount=target=. \
  --mount=type=cache,target=/root/.cache/go-build \
  make test
