aws_region := ap-northeast-1
ecr_host = $(aws_account_id).dkr.ecr.$(aws_region).amazonaws.com
downstream_repo_url = $(ecr_host)/enjoy-otel-downstream
upstream_repo_url = $(ecr_host)/enjoy-otel-upstream
collector_repo_url = $(ecr_host)/enjoy-otel-collector
image_tag := $(shell git rev-parse HEAD)

.PHONY: build-all
build-all: build-downstream build-upstream build-collector

.PHONY: build-downstream
build-downstream:
	docker build -f ./dockerfiles/downstream.Dockerfile -t $(downstream_repo_url):$(image_tag) .

.PHONY: build-upstream
build-upstream:
	docker build -f ./dockerfiles/upstream.Dockerfile -t $(upstream_repo_url):$(image_tag) .

.PHONY: build-collector
build-collector:
	docker build -f ./dockerfiles/otel-collector.Dockerfile -t $(collector_repo_url):$(image_tag) .

.PHONY: push-all
push-all: push-downstream push-upstream push-collector

.PHONY: push-downstream
push-downstream:
	docker push $(downstream_repo_url):$(image_tag)

.PHONY: push-upstream
push-upstream:
	docker push $(upstream_repo_url):$(image_tag)

.PHONY: push-collector
push-collector:
	docker push $(collector_repo_url):$(image_tag)

.PHONY: login
login:
	aws --region $(aws_region) ecr get-login-password | docker login --username AWS --password-stdin $(ecr_host)
