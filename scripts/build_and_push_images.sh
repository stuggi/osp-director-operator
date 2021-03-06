#!/bin/bash
set -ex

#
# Builds and pushes operator, bundle and index images for a given version of an operator
#
# NOTE: Requires opm and operator-sdk to be installed!
#

VERSION=${1:-"0.0.1"}
REPO=${2:-"quay.io/openstack-k8s-operators"}
OP_NAME=${3:-"osp-director-operator"}
IMG="$REPO/$OP_NAME":$VERSION
BUNDLE_IMG="$REPO/$OP_NAME-bundle":$VERSION
INDEX_IMG="$REPO/$OP_NAME-index:$VERSION"

# Base operator image
make manager
make manifests
make generate
IMG=${IMG} make docker-build docker-push

rm -Rf bundle
rm -Rf bundle.Dockerfile

# Bundle image
VERSION=${VERSION} IMG=${IMG} make bundle
VERSION=${VERSION} BUNDLE_IMG=${BUNDLE_IMG} make bundle-build

podman push ${BUNDLE_IMG}
#opm alpha bundle validate --tag ${BUNDLE_IMG} -b podman

# Index image
opm index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG} -u podman
podman push ${INDEX_IMG}