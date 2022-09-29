# Distribution of the CLI

This document explains the process by which the CLI is distrbuted to various package management systems, such as Homebrew, Chocolatey, and apt.

## Pipeline

The following process happens on each merge of a PR to main

```mermaid
graph LR
   Developer --> |merge PR| GM[Git Main] --> |merge trigger| RP[Release-Please updates changelog PR PR]
```

When the team decides to create a release, we do so by merging the changelog PR that  release-please has been keeping up to date.

This creates a new GitHub release, and also a Git Tag with the corresponding version number (e.g. `v2.1`)

```mermaid
graph LR
   Developer --> |merge changelog PR| GM[Git Main] --> |merge trigger| RP[Release-Please creates tag and github release]
```

Upon creation of the git tag, a `goreleaser` workflow runs, which builds the binaries and kicks off the distribution flow:

```mermaid
graph LR
   RP[release-please] --> |creates tag| GM[Git] --> |tag trigger| GO[GoReleaser]
```

The GoReleaser Github Actions workflow does most of the heavy lifting, as follows

## GoReleaser Github Workflow

```mermaid
flowchart TD
goreleaser --> msi --> generate-packages-and-publish

subgraph "goreleaser"
    build --> uploadToGHA
    
    build[Build CLI binaries for all architectures including deb, rpm and generate homebrew formula]
    uploadToGHA[Upload binaries+linux packages+homebrew formula to GHA artifact]
end
subgraph "msi"
    fetch --> buildmsi --> signmsi --> attachMSIToRelease --> uploadMSIToGHA

    fetch[Fetch GHA artifact]
    buildmsi[Build MSI installer]
    signmsi[Sign MSI installer]
    attachMSIToRelease["Attach MSI to GHA release"]
    uploadMSIToGHA["Upload(append) MSI to GHA artifact"]
end
subgraph generate-packages-and-publish
    fetch2 --> getScripts --> choco --> zipall --> octoPush --> octoRelease

    fetch2[Fetch MSI + CLI Binaries from GHA artifact]
    getScripts[Copy scripts to publish rpm and deb from OctopusDeploy/linux-package-feeds]
    choco[Create chocolatey package]
    zipall[Create octopus-cli-VERSION.zip with all packages and scripts]
    octoPush[Push octopus-cli-VERSION.zip to octopus deploy]
    octoRelease[Create release in octopus deploy using VERSION]
end
```

After which point Octopus is used to publish the packages to the external marketplaces, using the following deployment process.
The lifecycle is configured to automatically deploy upon release creation

## Octopus Deployment Process

```mermaid
flowchart LR
start[Fetch octopus-cli-VERSION.zip]
choco-push[Push CLI to chocolatey]
homebrew-pr[Create pull request to update homebrew]
apt-push[Publish to APT repo]
rpm-push[Publish to RPM repo]

start-->choco-push
start-->homebrew-pr
start-->apt-push
start-->rpm-push
```