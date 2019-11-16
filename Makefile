
ORG=dockereng
CONTROLLER_IMAGE_NAME=stack-controller
E2E_IMAGE_NAME=stack-e2e
TAG=latest # TODO work out versioning scheme
TEST_SCOPE?=./...
BUILD_ARGS= \
    --build-arg ALPINE_BASE=alpine:3.10.2 \
    --build-arg GOLANG_BASE=golang:1.12.8-alpine3.10

build:
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):$(TAG) .

test:
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):test --target unit-test .

lint:
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):lint --target lint .

standalone:
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):$(TAG) --target standalone .

integration:
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):integration --target integration .

it: integration
	- docker rm $(CONTROLLER_IMAGE_NAME)_it
	docker run -w /go/src/github.com/docker/stacks -v /var/run/docker.sock:/var/run/docker.sock -p 8080:2375 --name $(CONTROLLER_IMAGE_NAME)_it $(ORG)/$(CONTROLLER_IMAGE_NAME):integration
	docker cp $(CONTROLLER_IMAGE_NAME)_it:/itcover.out .
	docker rm $(CONTROLLER_IMAGE_NAME)_it
	go tool cover -html=itcover.out -o=itcoverage.html

# For developers...


# Get coverage results in a web browser
cover: test
	docker create --name $(CONTROLLER_IMAGE_NAME)_cover $(ORG)/$(CONTROLLER_IMAGE_NAME):test  && \
	    docker cp $(CONTROLLER_IMAGE_NAME)_cover:/cover.out . && docker rm $(CONTROLLER_IMAGE_NAME)_cover
	go tool cover -html=cover.out -o=coverage.html

build-mocks:
	@echo "Generating mocks"
	mockgen -package=mocks github.com/docker/stacks/pkg/interfaces BackendClient | sed s,github.com/docker/stacks/vendor/,,g > pkg/mocks/mock_backend.go
	mockgen -package=mocks github.com/docker/stacks/pkg/reconciler/reconciler Reconciler | sed s,github.com/docker/stacks/vendor/,,g > pkg/mocks/mock_reconciler.go
	mockgen -package=mocks github.com/docker/stacks/pkg/store ResourcesClient | sed s,github.com/docker/stacks/vendor/,,g > pkg/mocks/mock_resources_client.go

pkg/compose/schema/bindata.go: pkg/compose/schema/data/*.json
	docker build $(BUILD_ARGS) -t $(ORG)/$(CONTROLLER_IMAGE_NAME):build --target builder .
	docker create --name $(CONTROLLER_IMAGE_NAME)_schema $(ORG)/$(CONTROLLER_IMAGE_NAME):build && \
	    docker cp $(CONTROLLER_IMAGE_NAME)_schema:/go/src/github.com/docker/stacks/$@ $@ && docker rm $(CONTROLLER_IMAGE_NAME)_schema

.PHONY: e2e
