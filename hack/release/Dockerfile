# Copyright 2023 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

FROM ubuntu:22.04

COPY terramate /usr/local/bin/terramate

RUN apt-get -y update && apt-get install -y git

ENTRYPOINT ["terramate"]
