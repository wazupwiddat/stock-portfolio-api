# Define the default make action
.PHONY: all
all: build push

# Variables
BIN := bin
SERVER := server
SRC := cmd/main.go
REPO=026450499422.dkr.ecr.us-east-1.amazonaws.com/stock-portfolio-api
TAG=$(shell git rev-parse --short HEAD)  # Using git commit hash as image tag, ensure your directory is a git repository
IMAGE_NAME=$(REPO):$(TAG)

# Build the Go binary
.PHONY: build-go
build-go:
	@echo "Building Go binary..."
	go build -o $(BIN)/$(SERVER) $(SRC)

# Build the Docker image
.PHONY: build
build: build-go
	@echo "Building Docker image $(IMAGE_NAME)..."
	docker build -t $(IMAGE_NAME) .
	docker tag $(IMAGE_NAME) $(REPO):latest

# Log in to AWS ECR
.PHONY: login
login:
	@echo "Logging in to AWS ECR..."
	aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin $(REPO)

# Push the Docker image to AWS ECR
.PHONY: push
push: build login
	@echo "Pushing Docker image $(REPO):latest..."
	docker push $(REPO):latest

# Clean up the binaries
.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f main
