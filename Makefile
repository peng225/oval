BUILD_TARGET=oval

GO_FILES:=$(shell find . -type f -name '*.go' -print)
MINIO_DATAPATH:=~/minio/data

$(BUILD_TARGET): $(GO_FILES)
	CGO_ENABLED=0 go build -o $@ -v

.PHONY: test
test: $(BUILD_TARGET)
	go test -v ./...

.PHONY: local-run
local-run: $(BUILD_TARGET)
	docker run \
	   -p 9000:9000 \
	   -p 9090:9090 \
	   --name minio \
	   -v $(MINIO_DATAPATH):/data \
	   --rm -d \
	   quay.io/minio/minio server /data --console-address ":9090"
	./oval -size 4k-16k --time 5 -num_obj 1000 -num_worker 4 -bucket test-bucket -endpoint http://localhost:9000 -save test.json
	./oval --time 3 -load test.json
	docker stop minio

.PHONY: clean
clean:
	rm -f $(BUILD_TARGET)
