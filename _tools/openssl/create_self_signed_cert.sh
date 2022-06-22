#!/bin/sh

# SPDX-FileCopyrightText: 2022 NRK
#
# SPDX-License-Identifier: GPL-3.0-only

openssl req -x509 -newkey rsa:4096 -sha256 -utf8 -days 365 -nodes -config openssl.conf -keyout cert.key -out cert.crt
