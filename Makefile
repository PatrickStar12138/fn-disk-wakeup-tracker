.PHONY: build test release clean check-version

build:
	./scripts/build.sh all

test:
	./scripts/check-comments.sh
	go test ./...
	cd web && npm test -- --run

release:
	./scripts/build.sh all

clean:
	./scripts/clean.sh

check-version:
	./scripts/check-version.sh
