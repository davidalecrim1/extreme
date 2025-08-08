VERSION=v0.6.0

build:
	docker build -t davidalecrim1/extreme-proxy:$(VERSION) .

publish:
	docker buildx build --builder default \
		--platform linux/amd64,linux/arm64 \
		-t davidalecrim1/extreme-proxy:$(VERSION) \
		--push .