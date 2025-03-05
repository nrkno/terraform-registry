#!/bin/sh

# SPDX-FileCopyrightText: 2022 - 2025 NRK
#
# SPDX-License-Identifier: MIT

openssl req -x509 -newkey rsa:4096 -sha256 -utf8 -days 365 -nodes -config openssl.conf -keyout cert.key -out cert.crt
