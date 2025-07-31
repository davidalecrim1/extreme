build:
	docker build -t extreme-proxy:latest -t extreme-proxy:latest .

publish-amd64:
	docker buildx build --platform linux/amd64 -t extreme-proxy:latest -t extreme-proxy:latest . --push

publish-arm64:
	docker buildx build --platform linux/arm64 -t extreme-proxy:latest -t extreme-proxy:latest . --push