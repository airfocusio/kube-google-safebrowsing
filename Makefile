.PHONY: *

run:
	go run . -v --kube-config --interval 5s --additional-domains microsoftofficeonedrivefileshare.on.fleek.co

test:
	go test -v ./...

test-deps:
	./test/cluster/start.sh

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out

build:
	goreleaser release --rm-dist --skip-publish --snapshot

release:
	goreleaser release --rm-dist

regenerate-deploy:
	kustomize build deploy/kubernetes > deploy/kubernetes/manifests.yaml

