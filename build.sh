#!/bin/bash

# CNI Basic Plugin Build Script
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
BUILD_TYPE="release"
VERBOSE=false
CLEAN=false

# Print usage
usage() {
    echo "CNI Basic Plugin Build Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -t, --type TYPE      Build type: debug, release, or dev (default: release)"
    echo "  -v, --verbose        Enable verbose output"
    echo "  -c, --clean          Clean before building"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -t debug -v       Build debug version with verbose output"
    echo "  $0 -c -t release     Clean and build release version"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            BUILD_TYPE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            usage
            exit 1
            ;;
    esac
done

# Validate build type
case $BUILD_TYPE in
    debug|release|dev)
        ;;
    *)
        echo -e "${RED}Invalid build type: $BUILD_TYPE${NC}"
        echo "Valid types: debug, release, dev"
        exit 1
        ;;
esac

# Print build info
echo -e "${GREEN}CNI Basic Plugin Build Script${NC}"
echo "=============================="
echo "Build Type: $BUILD_TYPE"
echo "Verbose: $VERBOSE"
echo "Clean: $CLEAN"
echo ""

# Check dependencies
echo -e "${YELLOW}Checking dependencies...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}Go is not installed or not in PATH${NC}"
    exit 1
fi

if ! command -v make &> /dev/null; then
    echo -e "${RED}Make is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}Dependencies OK${NC}"

# Set verbose flag for make if requested
MAKE_FLAGS=""
if [ "$VERBOSE" = true ]; then
    MAKE_FLAGS="-s"
fi

# Clean if requested
if [ "$CLEAN" = true ]; then
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    make $MAKE_FLAGS clean
fi

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
make $MAKE_FLAGS deps

# Run checks
echo -e "${YELLOW}Running code quality checks...${NC}"
if [ "$BUILD_TYPE" = "release" ]; then
    make $MAKE_FLAGS check
else
    echo "Skipping checks for $BUILD_TYPE build"
fi

# Build based on type
echo -e "${YELLOW}Building $BUILD_TYPE version...${NC}"
case $BUILD_TYPE in
    debug)
        make $MAKE_FLAGS build-debug
        ;;
    release)
        make $MAKE_FLAGS build
        ;;
    dev)
        make $MAKE_FLAGS build
        ;;
esac

# Test build
if [ "$BUILD_TYPE" = "release" ]; then
    echo -e "${YELLOW}Testing built binary...${NC}"
    make $MAKE_FLAGS test-version
fi

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
echo ""

# Show build artifacts
if [ -d "bin" ]; then
    echo "Build artifacts:"
    ls -la bin/
fi

echo ""
echo "To install the plugin:"
echo "  make install"
echo ""
echo "To test all commands:"
echo "  make test-all-commands"