# SPDX-FileCopyrightText: 2022 NRK
#
# SPDX-License-Identifier: GPL-3.0-only

FROM golang:1.17-bullseye as build

RUN wget -O /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.5/dumb-init_1.2.5_x86_64 \
    && chmod +x /usr/local/bin/dumb-init

WORKDIR /go/src/app
ADD . /go/src/app

RUN go get -d -v ./...
RUN make build

FROM gcr.io/distroless/base-debian11
COPY --from=build /go/src/app/terraform-registry /
COPY --from=build /usr/local/bin/dumb-init /bin/
USER nonroot
ENTRYPOINT ["/bin/dumb-init", "--"]
CMD ["/terraform-registry"]
