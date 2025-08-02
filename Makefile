VERSION=v0.4.0

build:
	docker build -t davidalecrim1/extreme-proxy:$(VERSION) .

setup-buildx:
	docker buildx create --name mybuilder --use || true

publish: setup-buildx
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t davidalecrim1/extreme-proxy:$(VERSION) \
		--push .
