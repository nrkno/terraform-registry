# SPDX-FileCopyrightText: 2022 NRK
#
# SPDX-License-Identifier: GPL-3.0-only

CMD_SOURCE=./cmd/terraform-registry
BINARY_NAME=./terraform-registry

build:
	go build -o ${BINARY_NAME} ${CMD_SOURCE}

build-docker:
	docker build . -t terraform-registry

test:
	go test ./...

run:
	go run ${CMD_SOURCE}

reuse:
	find . -type f \
		| grep -vP '^(./.git|./.reuse|./LICENSES/|./terraform-registry)' \
		| grep -vP '(\.tf)$$' \
		| xargs reuse addheader --license GPL-3.0-only --copyright NRK --year `date +%Y` --skip-unrecognised
	find . -type f -name '*.tf' \
		| xargs reuse addheader --license GPL-3.0-only --copyright NRK --year `date +%Y` --style python

clean:
	go clean
	rm ${BINARY_NAME}
