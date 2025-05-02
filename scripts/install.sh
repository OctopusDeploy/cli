#!/usr/bin/env bash

# Some helpful functions
yell() { echo -e "${RED}FAILED> $* ${NC}" >&2; }
die() { yell "$*"; exit 1; }
try() { "$@" || die "failed executing: $*"; }
log() { echo -e "$*"; }

# Colors for colorizing
RED='\033[0;31m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
NC='\033[0m'

INSTALL_PATH=${INSTALL_PATH:-"/usr/local/bin"}
NEED_SUDO=0

function maybe_sudo() {
    if [[ "$NEED_SUDO" == '1' ]]; then
        sudo "$@"
    else
        "$@"
    fi
}

# check for curl
hasCurl=$(which curl)
if [ "$?" = "1" ]; then
    die "You need to install curl to use this script."
fi

log "Selecting version..."

version=${VERSION:-v2.16.0}

if [ ! $version ]; then
    log "${YELLOW}"
    log "Failed while attempting to install octopus cli. Please manually install:"
    log ""
    log "1. Open your web browser and go to https://github.com/OctopusDeploy/cli/releases"
    log "2. Download the cli from the latest release for your platform. Make sure rename it octopus"
    log "3. chmod +x ./octopus"
    log "4. mv ./octopus /usr/local/bin"
    log "${NC}"
    die "exiting..."
fi

log "Selected version: $version"

log "${YELLOW}"
log NOTE: Install a specific version of the CLI by using VERSION variable
log 'curl -L https://github.com/OctopusDeploy/cli/raw/scripts/install.sh | VERSION=v0.4.0 bash'
log "${NC}"

# check for existing octopus cli installation
hasCli=$(which octopus)
if [ "$?" = "0" ]; then
    log ""
    log "${GREEN}You already have the octopus cli at '${hasCli}'${NC}"
    export n=3
    log "${YELLOW}Downloading again in $n seconds... Press Ctrl+C to cancel.${NC}"
    log ""
    sleep $n
fi

# get platform and arch
platform='unknown'
unamestr=`uname`
if [[ "$unamestr" == 'Linux' ]]; then
    platform='linux'
elif [[ "$unamestr" == 'Darwin' ]]; then
    platform='macOS'
fi

if [[ "$platform" == 'unknown' ]]; then
    die "Unknown OS platform"
fi

arch='unknown'
archstr=`uname -m`
if [[ "$archstr" == 'x86_64' ]]; then
    arch='amd64'
elif [[ "$archstr" == 'arm64' ]] || [[ "$archstr" == 'aarch64' ]]; then
    arch='arm64'
else
    die "prebuilt binaries for $(arch) architecture not available, please try building from source https://github.com/OctopusDeploy/cli"
fi

# some variables
suffix="_${version//v}_${platform}_${arch}.tar.gz"
targetFile="/tmp/octopus$suffix"
targetExe="/tmp/octopus"
targetChangeLog="/tmp/CHANGELOG.md"
targetLicense="/tmp/LICENSE"
targetReadme="/tmp/README.md"

if [ -e $targetFile ]; then
    rm $targetFile
fi

if [ -e $targetExe ]; then
  rm $targetExe
fi

if [ -e $targetChangeLog ]; then
  rm $targetChangeLog
fi

if [ -e $targetLicense ]; then
  rm $targetLicense
fi

if [ -e $targetReadme ]; then
  rm $targetReadme
fi

log "${PURPLE}Downloading Octopus cli for $platform-$arch to ${targetFile}${NC}"
url=https://github.com/OctopusDeploy/cli/releases/download/$version/octopus$suffix

try curl -L# -f -o $targetFile "$url"
try tar -xzf $targetFile -C "/tmp"
try chmod +x $targetExe

log "${GREEN}Download complete!${NC}"

# check for sudo
needSudo=$(mkdir -p ${INSTALL_PATH} && touch ${INSTALL_PATH}/.octopusinstall &> /dev/null)
if [[ "$?" == "1" ]]; then
    NEED_SUDO=1
fi
rm ${INSTALL_PATH}/.octopusinstall &> /dev/null

if [[ "$NEED_SUDO" == '1' ]]; then
    log
    log "${YELLOW}Path '$INSTALL_PATH' requires root access to write."
    log "${YELLOW}This script will attempt to execute the move command with sudo.${NC}"
    log "${YELLOW}Are you ok with that? (y/N)${NC}"
    read a
    if [[ $a == "Y" || $a == "y" || $a = "" ]]; then
        log
    else
        log
        log "  ${BLUE}sudo mv $targetFile ${INSTALL_PATH}/octopus${NC}"
        log
        die "Please move the binary manually using the command above."
    fi
fi

log "Moving Octopus cli from $targetExe to ${INSTALL_PATH}"

try maybe_sudo mv $targetExe ${INSTALL_PATH}/octopus

log
log "${GREEN}Octopus cli installed to ${INSTALL_PATH}${NC}"
log

if [ -e $targetFile ]; then
    rm $targetFile
fi

if [ -e $targetExe ]; then
  rm $targetExe
fi

if [ -e $targetChangeLog ]; then
  rm $targetChangeLog
fi

if [ -e $targetLicense ]; then
  rm $targetLicense
fi

if [ -e $targetReadme ]; then
  rm $targetReadme
fi

log "Running ${BLUE}octopus version${NC}"
octopus version

if ! $(echo "$PATH" | grep -q "$INSTALL_PATH"); then
    log
    log "${YELLOW}$INSTALL_PATH not found in \$PATH, you might need to add it${NC}"
    log
fi

