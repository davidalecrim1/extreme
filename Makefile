VERSION=v1.2.0-backend-api-aware

build:
	docker build -t davidalecrim1/extreme-proxy:$(VERSION) .

publish:
	docker buildx build \
	--platform linux/amd64 \
	-t davidalecrim1/extreme-proxy:$(VERSION) \
	--push \
	-f Dockerfile .
	
	docker buildx build \
	--platform linux/arm64 \
	-t davidalecrim1/extreme-proxy:$(VERSION) \
	--push \
	-f Dockerfile .