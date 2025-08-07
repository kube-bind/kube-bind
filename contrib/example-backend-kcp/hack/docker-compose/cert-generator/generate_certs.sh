#!/usr/bin/env bash

# Copyright 2025 The Kube Bind Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

CERT_DIR=/certs
CERT_NAME=dex

echo "Generating TLS certificates using genkey..."

# Generate certificates using genkey
genkey $CERT_NAME

# Rename or move the generated files if necessary
# Assuming genkey outputs CERT_NAME.pem and CERT_NAME.key.pem in the current directory
mv ${CERT_NAME}.pem ${CERT_DIR}/${CERT_NAME}.pem
mv ${CERT_NAME}.key ${CERT_DIR}/${CERT_NAME}.key
mv ${CERT_NAME}.crt ${CERT_DIR}/${CERT_NAME}.crt

chmod 644 ${CERT_DIR}/${CERT_NAME}.*

echo "Certificates generated successfully at ${CERT_DIR}/"


sleep infinity