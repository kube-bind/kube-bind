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

get_host_ip() {
  local os
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  if [[  "$os" == linux ]]; then
    export HOST_IP="$(hostname -i | cut -d' ' -f1)"
  elif [[ "$os" == darwin ]]; then
    export HOST_IP="$(ifconfig | grep "inet " | grep -v 127.0.0.1 | cut -d\  -f2)"
  fi
}
