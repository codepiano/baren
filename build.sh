#!/usr/bin/env bash

set -xe

for plugin in `find src/plugins -name "*.go"`;
do
    plugin_file="${plugin##*/}"
    plugin_path="${plugin%/*}"
    plugin_name="${plugin_file%%.*}"
    go build -buildmode plugin -o "${plugin_path#src/}/$plugin_name.so" "$plugin"
done
