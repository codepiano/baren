#!/usr/bin/env bash

set -xe

for plugin in `find plugins_src -name "*.go"`;
do
    plugin_file="${plugin##*/}"
    plugin_path="${plugin%/*}"
    plugin_path="${plugin_path#*/}"
    plugin_name="${plugin_file%%.*}"
    go build -buildmode plugin -o "plugins/${plugin_path}/$plugin_name.so" "$plugin"
done
