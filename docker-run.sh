#!/bin/bash

# Check if path argument is provided
if [ -z "$1" ]; then
    echo "Usage: ./docker-run.sh /path/to/your/videos"
    exit 1
fi

# Convert to absolute path
VIDEO_PATH=$(realpath "$1")

# Check if directory exists
if [ ! -d "$VIDEO_PATH" ]; then
    echo "Error: Directory $VIDEO_PATH does not exist"
    exit 1
fi

docker run \
    --name govideoconverter \
    -v "$VIDEO_PATH:/videos" \
    --rm \
    govideoconverter:latest "$@"
