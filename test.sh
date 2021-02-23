#!/usr/bin/env bash

set -xe

echo "subject: test\nhi" | curl -vvv --url 'smtp://localhost:2526' \
  --mail-from 'from@mailway.app' --mail-rcpt "to@mailway.app" \
  --upload-file -

