#!/bin/bash
set -e
echo "Running Installer SHA verification tests on Linux..."
go run ./cmd/main -cli -project "test" -validate-sha
