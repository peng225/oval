OVAL=./oval
IMAGE_NAME ?= ghcr.io/peng225/oval

GO_FILES:=$(shell find . -type f -name '*.go' -print)
MINIO_DATAPATH:=~/minio/data

EXEC_TIME?=5s

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
	$(OVAL) --size 4k-12m --time $(EXEC_TIME) --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --ope_ratio 8,8,8,1 --endpoint http://localhost:9000 --multipart_thresh 5m --save test.json
	$(OVAL) --time 3s --multipart_thresh 6m --load test.json

.PHONY: run-multi-process
run-multi-process: $(OVAL)
	make run-followers
	sleep 1
	make run-leader

.PHONY: run-leader
run-leader: $(OVAL)
	$(OVAL) leader --follower_list "http://localhost:8080,http://localhost:8081,http://localhost:8082" --size 4k-12m --time $(EXEC_TIME) --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --ope_ratio 8,8,8,1 --endpoint http://localhost:9000 --multipart_thresh 5m

.PHONY: run-followers
run-followers: $(OVAL)
	$(OVAL) follower --follower_port 8080 &
	$(OVAL) follower --follower_port 8081 &
	$(OVAL) follower --follower_port 8082 &

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
