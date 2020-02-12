#!/usr/bin/env bash
base_dir=$(dirname $(cd $(dirname "$0") && pwd -P)/$(basename "$0"))
now=$(pwd)
cd "${base_dir}"/pkg/templates
go-bindata -pkg templates data
cd "$now"