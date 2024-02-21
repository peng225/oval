OVAL := ./oval
IMAGE_NAME ?= ghcr.io/peng225/oval

BINDIR := bin

GO_FILES := $(shell find . -path './test' -prune -o -type f -name '*.go' -print)
MINIO_DATADIR := $(shell git rev-parse --show-toplevel)/test/data
MINIO_CERTDIR := $(shell git rev-parse --show-toplevel)/test/certs

CERTGEN_VERSION := v1.2.1
CERTGEN := $(BINDIR)/certgen-$(CERTGEN_VERSION)

GOLANGCI_LINT_VERSION := v1.55.1
GOLANGCI_LINT := $(BINDIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)

S3_ENDPOINT ?= http://localhost:9000
CERT_CONFIG ?=

EXEC_TIME ?= 5s
COMMON_OPTIONS := --size 4k-12m --time $(EXEC_TIME) --num_obj 1024 --num_worker 4 --bucket "test-bucket,test-bucket2" --ope_ratio 8,8,8,1 --endpoint $(S3_ENDPOINT) --multipart_thresh 5m

$(OVAL): $(GO_FILES)
	CGO_ENABLED=0 go build -o $@ -v

$(BINDIR):
	mkdir -p $@

$(MINIO_DATADIR):
	mkdir -p $@

$(MINIO_CERTDIR):
	mkdir -p $@

.PHONY: image
image:
	docker build . --file Dockerfile --tag $(IMAGE_NAME)

$(GOLANGCI_LINT): | $(BINDIR)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b . $(GOLANGCI_LINT_VERSION)
	mv golangci-lint $(GOLANGCI_LINT)

.PHONY: lint
lint: | $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

.PHONY: test
test: $(OVAL)
	go test -v ./...

.PHONY: run
run: $(OVAL)
	$(OVAL) $(COMMON_OPTIONS) $(CERT_CONFIG) --save test.json
	$(OVAL) --time 3s --multipart_thresh 6m --load test.json $(CERT_CONFIG)

.PHONY: run-multi-process
run-multi-process: $(OVAL)
	make run-followers
	sleep 1
	make run-leader

.PHONY: run-leader
run-leader: $(OVAL)
	$(OVAL) leader --follower_list "http://localhost:8080,http://localhost:8081,http://localhost:8082"\
		$(COMMON_OPTIONS)

.PHONY: run-leader-with-config
run-leader-with-config: $(OVAL)
	$(OVAL) leader --config test_config.json $(COMMON_OPTIONS)

.PHONY: run-followers
run-followers: $(OVAL)
	$(OVAL) follower --follower_port 8080 $(CERT_CONFIG) &
	$(OVAL) follower --follower_port 8081 $(CERT_CONFIG) &
	$(OVAL) follower --follower_port 8082 $(CERT_CONFIG) &

.PHONY: run-and-signal
run-and-signal: $(OVAL)
	$(OVAL) $(COMMON_OPTIONS) $(CERT_CONFIG) &
	sleep 2
	kill $$(pidof $(OVAL))
	wait
	$(OVAL) $(COMMON_OPTIONS) $(CERT_CONFIG) &
	sleep 2
	kill -2 $$(pidof $(OVAL))
	wait

.PHONY: run-leader-and-signal-follower
run-leader-and-signal: $(OVAL)
	$(OVAL) leader --follower_list "http://localhost:8080,http://localhost:8081,http://localhost:8082"\
		$(COMMON_OPTIONS) &
	sleep 2
	FOLLOWER_PID=$$(ps aux | grep oval | grep follower_port | awk '{print $$2}' | head -n 1); \
	kill $${FOLLOWER_PID}
	wait

$(CERTGEN): | $(BINDIR)
	wget https://github.com/minio/certgen/releases/download/$(CERTGEN_VERSION)/certgen-linux-amd64
	mv certgen-linux-amd64 $@
	chmod +x $@

.PHONY: keypair
keypair: $(CERTGEN) $(MINIO_CERTDIR)
	$(CERTGEN) -host "127.0.0.1,localhost"
	mv public.crt $(MINIO_CERTDIR)
	mv private.key $(MINIO_CERTDIR)

.PHONY: start-minio
start-minio: | $(MINIO_DATADIR)
	docker run \
	   --user $$(id -u):$$(id -g) \
	   -p 9000:9000 \
	   -p 9090:9090 \
	   --name minio \
	   -v $(MINIO_DATADIR):/data \
	   --rm -d \
	   quay.io/minio/minio server /data --console-address ":9090"

.PHONY: start-minio-https
start-minio-https: keypair | $(MINIO_DATADIR)
	docker run \
	   -p 9000:9000 \
	   -p 9090:9090 \
	   --name minio \
	   -v $(MINIO_DATADIR):/data \
	   -v $(MINIO_CERTDIR):/certs \
	   --rm -d \
	   quay.io/minio/minio server /data --console-address ":9090" --certs-dir /certs

.PHONY: stop-minio
stop-minio:
	docker stop minio

.PHONY: clean
clean:
	rm -f $(OVAL) test.json
