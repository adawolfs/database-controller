#!/usr/bin/env bash
vendor/k8s.io/code-generator/generate-groups.sh all \
github.com/adawolfs/database-controller/pkg/client \
github.com/adawolfs/database-controller/pkg/apis \
adawolfs.com:v1