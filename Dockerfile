# SPDX-FileCopyrightText: 2022 - 2025 NRK
#
# SPDX-License-Identifier: MIT

FROM golang:1.24-bookworm as build

RUN wget -O /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_x86_64 \
    && chmod +x /usr/local/bin/dumb-init

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download -x

COPY . /go/src/app
RUN make GO_FLAGS="-buildvcs=false" test build

FROM gcr.io/distroless/base-debian12
COPY --from=build /go/src/app/terraform-registry /bin/
COPY --from=build /usr/local/bin/dumb-init /bin/
USER nonroot
ENTRYPOINT ["/bin/dumb-init", "--", "/bin/terraform-registry"]
CMD ["-help"]
