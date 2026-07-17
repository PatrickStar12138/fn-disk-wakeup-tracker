.PHONY: build test release clean check-version

build:
	./scripts/build.sh all

test:
	./scripts/check-comments.sh
	go test ./...
	cd web && npm test -- --run
	for script in packaging/fnos/cmd/* scripts/*.sh tests/*.sh; do bash -n "$$script"; done
	bash tests/lifecycle_test.sh
	bash tests/package_permissions_test.sh

release:
	./scripts/build.sh all

clean:
	./scripts/clean.sh

check-version:
	./scripts/check-version.sh
