#!/bin/bash
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