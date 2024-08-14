# SPDX-FileCopyrightText: 2022 - 2024 NRK
#
# SPDX-License-Identifier: GPL-3.0-only

BINARY_NAME := ./terraform-registry
CMD_SOURCE  := ./cmd/terraform-registry
DOCKER_TAG  := terraform-registry

.PHONY: all
all : reuse build test

.PHONY: build
build :
	go build -ldflags "$(LD_FLAGS)" $(GO_FLAGS) -o $(BINARY_NAME) $(CMD_SOURCE)

.PHONY: build-docker
build-docker :
	docker build . -t $(DOCKER_TAG)

.PHONY: test
test :
	go test ./... -timeout 10s

.PHONY: run
run :
	go run $(CMD_SOURCE)

.PHONY: reuse
reuse :
	find . -type f \
		| grep -vP '^(./.git|./.reuse|./LICENSES/|./terraform-registry)' \
		| grep -vP '(/\.git/|/\.terraform/)' \
		| grep -vP '(\\.tf)$$' \
		| xargs reuse annotate --merge-copyrights --license GPL-3.0-only --copyright NRK --year `date +%Y` --skip-unrecognised
		find . -type f -name '*.tf' \
		| grep -vP '(/\.git/|/\.terraform/)' \
		| xargs reuse annotate --merge-copyrights --license GPL-3.0-only --copyright NRK --year `date +%Y` --style python

.PHONY: clean
clean :
	go clean
	rm $(BINARY_NAME)
