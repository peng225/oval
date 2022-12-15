OVAL=./oval
IMAGE_NAME ?= ghcr.io/peng225/oval

GO_FILES:=$(shell find . -type f -name '*.go' -print)
MINIO_DATAPATH:=~/minio/data

$(OVAL): $(GO_FILES)
	CGO_ENABLED=0 go build -o $@ -v

.PHONY: image
image:
	docker build . --file Dockerfile --tag $(IMAGE_NAME)

.PHONY: test
test: $(OVAL)
	go test -v ./...

.PHONY: run
run: $(OVAL)
	$(OVAL) -size 4k-16k -time 5 -num_obj 1000 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000 -save test.json
	$(OVAL) -time 3 -load test.json

.PHONY: start-minio
start-minio: $(OVAL)
	docker run \
	   -p 9000:9000 \
	   -p 9090:9090 \
	   --name minio \
	   -v $(MINIO_DATAPATH):/data \
	   --rm -d \
	   quay.io/minio/minio server /data --console-address ":9090"

.PHONY: stop-minio
stop-minio:
	docker stop minio

.PHONY: clean
clean:
	rm -f $(OVAL) test.json
