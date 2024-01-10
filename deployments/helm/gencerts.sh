#!/usr/bin/env bash

# Copyright (C) 2023 Intel Corporation
# SPDX-License-Identifier: Apache-2.0

###############################################################################
# Function to generate a private key excatadmission-tls.key and certificate 
# excatadmission-tls.crt signed by the root CA
# Arguments:
# $1: path containing root CA key and cert
# $2: hostname to be used as subject
# $3: validity of valid certificate
# Returns:
#   None
###############################################################################
function generate_tls_key_and_cert() {
  local path=$1
  local hostname=$2
  local validity=$3


  echo "{INFO}: generating private key and certificate signed by the root CA in ${path}"
  openssl genrsa -out ${path}/excatadmission-tls.key 3072
  openssl req -new -key ${path}/excatadmission-tls.key -subj "/CN=${hostname} " -config ${path}/openssl.cnf \
   | openssl x509 -days ${validity} -req -CA ${path}/ca.crt -CAkey ${path}/ca.key -sha512 -CAcreateserial -out ${path}/excatadmission-tls.crt -extensions v3_req -extfile ${path}/openssl.cnf
  if [ ! -f "${path}"/excatadmission-tls.crt ]; then
    echo "{ERROR}: certificate not generated"
    return 1
  fi
  return 0
}

###############################################################################
# Function to generate a private key ca.key and based on that a self signed 
# root certificate ca.crt
# Arguments:
# $1: Destination folder
# Returns:
#   None
###############################################################################
function generate_key_and_ca() {
  local path=$1


  mkdir -p "${path}" || {
    echo "{ERROR}: Error creating path: ${path}"
    return 1
  }
	chmod 0700 ${path}
  echo "{INFO}: Creating private key and self signed CA in ${path}"
  openssl req -nodes -new -x509 -sha512 -pkeyopt rsa_keygen_bits:3072 -keyout "${path}"/ca.key -out "${path}"/ca.crt -subj "/CN=Excat admission controller CA"
  if [ ! -f "${path}"/ca.crt ]; then
    echo "{ERROR}: self signed certificate was not generated."
    return 1
  fi

  return 0
}

###############################################################################
# Function to generate TLS OpenSSL config file openssl.cnf.
# Arguments:
# $1: Destination folder
# $2: Hostname
# Returns:
#   None
###############################################################################
function generate_tls_config() {
  local path=$1
  local hostname=$2
  local dst_path="${path}"/openssl.cnf


  echo "{INFO}: Creating TLS config file in ${path}"

  cat > "${dst_path}" <<EOF
[ req ]
distinguished_name = req_distinguished_name
req_extensions = v3_req
default_md = sha512
default_bits = 3072
prompt = no
[ req_distinguished_name ]
CN = ${hostname}
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names
[ alt_names ]
DNS.1 = ${hostname}
EOF

  if [ ! -f $dst_path ]; then
    echo "{ERROR}: OpenSSL CSL TLS config file not generated"
    return 1
  fi

  return 0
}

: ${1?'missing destination directory'}
: ${2?'missing application name'}
: ${3?'missing namespace'}

dest_dir="$1"
name="$2"
namespace="$3"
if [ -z "$4" ]; then
  validity="365"
else
  validity="$4"
fi

generate_key_and_ca "${dest_dir}" 
echo "{INFO}: ${dest_dir}/ca.key and ${dest_dir}/ca.crt generated"
hostname="${name}.${namespace}.svc"
generate_tls_config "${dest_dir}" "${hostname}"
generate_tls_key_and_cert "${dest_dir}" "${hostname}" "${validity}"
echo "{INFO}: ${dest_dir}/excatadmission-tls.key and ${dest_dir}/excatadmission-tls.crt generated"
