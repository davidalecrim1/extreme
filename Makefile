build:
	docker build -t extreme-proxy:latest .

publish-amd64:
	docker buildx build --platform linux/amd64 -t davidalecrim1/extreme-proxy:v.0.1.0 . --push

publish-arm64:
	docker buildx build --platform linux/arm64 -t davidalecrim1/extreme-proxy:v.0.1.0 . --push

publish-all:
	make build
	make publish-amd64
	make publish-arm64