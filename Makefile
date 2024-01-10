# Copyright (C) 2023 Intel Corporation
# SPDX-License-Identifier: Apache-2.0
APP=csl-excat
NAMESPACE=excat
VER=v0.1.0
BUILDDIR=build
APPDIR=cmd
DEVICEPLUGINCNTFILE=deployments/images/deviceplugin/Containerfile
ADMISSIONCONTROLLERCNTFILE=deployments/images/admissioncontroller/Containerfile
TAG ?=
MINIKUBE_CLUSTER_NAME ?= test-cluster

.PHONY: setup build clean image image2cluster test helm unhelm srcpackage test-helm test-unhelm help

## setup: 1st time set up of the dev environment
setup:
	@echo "\n1st time setup..."
	@echo "Make sure to install... "
	@echo " * Podman - as explained in https://podman.io/getting-started/installation"
	@echo " * Helm - as explained in https://helm.sh/docs/intro/install"
	sudo apt install -y pandoc
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	pip install pre-commit
	pre-commit install

## build: cleans up and builds the go code
build: clean
	@echo "\nBuilding..."
	CGO_ENABLED=0 go build -o ${BUILDDIR} ./...

## buildincnt: build inside golang container
buildincnt:
	podman run -ti --rm -v .:/go/csl-excat docker.io/library/golang:latest bash -c 'go env -w GO111MODULE=off; cd csl-excat; export CGO_ENABLED=0; make build'

## test: run unit tests
test:
	go test -v ./...

## clean: cleans the image and binary
clean:
	@echo "\nCleaning up..."
	go clean

cleanimages:
	rm -rf ${BUILDDIR}/*
	if [ `podman images | awk '/${APP}-deviceplugin/&&/${VER}/ {print $$1":"$$2}'` ]; then \
	  podman rmi `podman images | awk '/${APP}-deviceplugin/&&/${VER}/ {print $$1":"$$2}'` ; \
	fi
	if [ `podman images | awk '/${APP}-admission/&&/${VER}/ {print $$1":"$$2}'` ]; then \
	  podman rmi `podman images | awk '/${APP}-admission/&&/${VER}/ {print $$1":"$$2}'` ; \
	fi

## image: cleans, builds the go code and builds an image with podman
image: cleanimages build
	@echo "\nBuilding image..."
	podman build --no-cache -t ${APP}-deviceplugin:${VER} -f ${DEVICEPLUGINCNTFILE} ${BUILDDIR}
	podman save --format docker-archive -o ${BUILDDIR}/${APP}-deviceplugin.tar ${APP}-deviceplugin:${VER}
	podman build --no-cache -t ${APP}-admission:${VER} -f ${ADMISSIONCONTROLLERCNTFILE} ${BUILDDIR}
	podman save --format docker-archive -o ${BUILDDIR}/${APP}-admission.tar ${APP}-admission:${VER}

## image2cluster: builds the image and adds it to the current host. Can be used if the the cri is containerd and the host is part of the cluster. Requires `ctr`.
image2cluster: image
	@echo "\nAdding image to current host..."
	sudo ctr -n k8s.io images import ${BUILDDIR}/${APP}-deviceplugin.tar
	sudo ctr -n k8s.io images import ${BUILDDIR}/${APP}-admission.tar

## helm: deploys the helm chart to a cluster. Requires to run `image` first
helm:
	@echo "\nDeploying helm chart..."
	./deployments/helm/gencerts.sh ./deployments/helm/certs ${APP}-admission ${NAMESPACE}
	helm install --namespace ${NAMESPACE} --create-namespace ${APP} deployments/helm

## unhelm: uninstalls the helm release installed before with `make helm`
unhelm:
	@echo "\nUninstalling helm chart..."
	helm uninstall --namespace ${NAMESPACE} ${APP}

## srcpackage: create src tar.gz package
srcpackage:
	echo -e "\nPackage src release with tag $$TAG" && \
	cd ./build && \
	SRCNAME=csl-excat-src-$$TAG && \
	mkdir -p $$SRCNAME && \
	cd $$SRCNAME && \
	cp -r ../../assets . && \
	cp -r ../../cmd . && \
	cp -r ../../deployments . && \
	cp -r ../../docs . && \
	cp -r ../../pkg . && \
	cp ../../LICENSE . && \
	cp ../../Makefile . && \
	cp ../../README.md . && \
	cp ../../go.mod . && \
	cp ../../go.sum . && \
	cd .. && \
	tar -czvf $$SRCNAME.tar.gz ./$$SRCNAME/

## test-setup: creates a minikube cluster with 2 VM-nodes using kvm2
test-setup:
	@echo "\nmake sure the following components are installed:"
	@echo " 1) minikube: https://minikube.sigs.k8s.io/docs/start/"
	@echo " 2) kvm: https://help.ubuntu.com/community/KVM/Installation"
	@echo " and set minikube to use the kvm2 driver with `minikube config set driver kvm2`"
	@echo "\n start cluster"
	minikube start --nodes 2 -p ${MINIKUBE_CLUSTER_NAME} --driver kvm2

## test-clean-deploy: removes excat images from the minikube test-cluster
test-clean-deploy:
	minikube image rm docker.io/localhost/${APP}-deviceplugin:${VER} -p ${MINIKUBE_CLUSTER_NAME}
	minikube image rm docker.io/localhost/${APP}-admission:${VER}    -p ${MINIKUBE_CLUSTER_NAME}

## test-image-deploy: loads excat images into the minikube test-cluster
test-image-deploy: image
	minikube image load ${BUILDDIR}/${APP}-deviceplugin.tar -p ${MINIKUBE_CLUSTER_NAME} && \
	minikube image load ${BUILDDIR}/${APP}-admission.tar -p ${MINIKUBE_CLUSTER_NAME}

## test-helm: installs the excat helm chart into the minikube test-cluster
test-helm:
	@echo "\nNOTE: depending on the minikube version, you may have to rename `master` to `control-plane` in the values.yaml file"
	@echo "\nDeploying helm chart..."
	cd deployments/helm/ && \
	./gencerts.sh certs ${APP}-admission ${NAMESPACE} && \
	helm install ${APP} --create-namespace -n ${NAMESPACE} . \
	--set tlsSecret.certSource="file" \
	--set deviceplugin.image.repository=localhost/${APP}-deviceplugin --set deviceplugin.image.tag=${VER} \
	--set admission.image.repository=localhost/${APP}-admission --set admission.image.tag=${VER}

## test-unhelm: uninstalls the excat helm chart
test-unhelm:
	@echo "\nUninstalling helm chart..."
	helm uninstall ${APP} -n ${NAMESPACE} \

## test-destroy: deletes the minikube test-cluster
test-destroy:
	minikube delete -p ${MINIKUBE_CLUSTER_NAME}

## help: prints this help message
help:
	@echo "Usage: \n"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
