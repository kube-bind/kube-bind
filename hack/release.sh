#!/usr/bin/env bash

# Copyright 2024 The Kube Bind Authors.
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

# This script ensures that the generated client code checked into git is up-to-date
# with the generator. If it is not, re-generate the configuration to update it.

set -o errexit
set -o nounset
set -o pipefail

# Usage helper function
usage() {
  echo "Usage: $0 <version_tag> \"comment\" [remote_name]"
  exit 1
}

# Check if at least the version tag and comment are provided
if [ $# -lt 2 ]; then
  usage
fi

VERSION_TAG=$1
RELEASE_COMMENT=$2
REMOTE_NAME=${3:-origin}  # Set default remote name to 'origin' if not provided

# Validate the version tag format
if ! [[ $VERSION_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: Version tag must be in 'vX.Y.Z' format."
  usage
fi

# Create the tag
git tag -a $VERSION_TAG -s -m "$RELEASE_COMMENT" 
git tag -a sdk/$VERSION_TAG -s -m "$RELEASE_COMMENT"
git tag -a sdk/kcp/$VERSION_TAG -s -m "$RELEASE_COMMENT"

# Check if the tag was created successfully
if [ $? -ne 0 ]; then
  echo "Error: Failed to create tag."
  exit 1
fi

# Push the tag to remote repository
git push $REMOTE_NAME $VERSION_TAG
git push $REMOTE_NAME sdk/$VERSION_TAG
git push $REMOTE_NAME sdk/kcp/$VERSION_TAG

# Check if the push was successful
if [ $? -ne 0 ]; then
  echo "Error: Failed to push the tag to the remote repository."
  exit 1
fi

echo "Version $VERSION_TAG has been successfully tagged and pushed to $REMOTE_NAME with comment: '$RELEASE_COMMENT'"
