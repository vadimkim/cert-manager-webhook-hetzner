IMAGE_NAME := "cert-manager-webhook-hosting-de"
IMAGE_TAG := "latest"

OUT := $(shell pwd)/deploy

$(shell mkdir -p "$(OUT)")

verify:
	go test -v .

build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml:
	helm template \
	    cert-manager-webhook-hosting-de \
        --set image.repository=$(IMAGE_NAME) \
        --set image.tag=$(IMAGE_TAG) \
		--namespace cert-manager \
        deploy/cert-manager-webhook-oci > "$(OUT)/rendered-manifest.yaml"
