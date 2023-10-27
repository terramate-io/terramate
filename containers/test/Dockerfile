# Copyright 2023 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

FROM golang:1.20-alpine3.16 AS base
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download


FROM base AS test

# Needed for go test -race
RUN apk add --no-cache git gcc g++
ENV CGO_ENABLED=1

RUN --mount=target=. \
  --mount=type=cache,target=/root/.cache/go-build \
  go test -race -count=1 ./cmd/terramate/e2etests
