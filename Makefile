VERSION=v1.1.2

build:
	docker build -t davidalecrim1/extreme-proxy:$(VERSION) .

publish-amd64:
	docker buildx build \
	--platform linux/amd64 \
	-t davidalecrim1/extreme-proxy:$(VERSION) \
	--push \
	-f Dockerfile .

publish-arm64:
	docker buildx build \
	--platform linux/arm64 \
	-t davidalecrim1/extreme-proxy:$(VERSION) \
	--push \
	-f Dockerfile .