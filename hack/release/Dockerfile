# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

FROM ubuntu:23.04

COPY terramate /usr/local/bin/terramate

RUN apt-get -y update && apt-get install -y git \
&& apt-get clean && rm -rf /var/lib/apt/lists/*

ENTRYPOINT ["terramate"]
