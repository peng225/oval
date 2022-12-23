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
	$(OVAL) -size 4k-16k -time 5 -num_obj 1024 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000 -save test.json
	$(OVAL) -time 3 -load test.json

.PHONY: run-multi-process
run-multi-process: $(OVAL)
	$(OVAL) -follower -follower_port 8080 &
	$(OVAL) -follower -follower_port 8081 &
	sleep 1
	$(OVAL) -leader -follower_list "http://localhost:8080,http://localhost:8081" -size 4k-16k -time 5 -num_obj 1024 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000

.PHONY: start-minio
start-minio:
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
