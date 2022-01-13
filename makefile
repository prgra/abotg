DEFAULT: docker

docker: ## Build docker images
	docker buildx build --build-arg BUILDPLATFORM=linux/amd64 --build-arg GOOS=linux --build-arg GOARCH=amd64 --platform=linux/amd64 --push -t  prgr/abotg:latest .
	docker buildx build --build-arg BUILDPLATFORM=linux/arm64 --build-arg GOOS=linux --build-arg GOARCH=arm64 --platform=linux/arm64 --push -t  prgr/abotg:arm64 .