#!/bin/bash

# Launch this script from the project root.

function command_exists() {
	command -v "$1" >/dev/null 2>&1
}

if ! command_exists "2goarray"; then
    echo "Installing 2goarray..."
    if go get -u github.com/cratonica/2goarray
    then
        echo Failure executing go get github.com/cratonica/2goarray
        exit 1
    fi
fi

for image in assets/tray-agent/icons/tray-bar/*.png; do
    name=$(basename "${image}" .png)
    2goarray "${name}" icon < "${image}" > "internal/tray-agent/icon/${name}.go"
done

gofmt -w internal/tray-agent/icon/Liqo*.go