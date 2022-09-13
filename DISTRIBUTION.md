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
subgraph "goreleaser"
    build[Build CLI binaries for all architectures]
    debs["Build linux packages (.deb, .rpm)"]
    upload1[Upload binaries+linux packages to GHA artifacts.zip]

    build-->debs-->upload1
end
subgraph "msi"
    fetch[Fetch GHA artifacts.zip]
    buildmsi[Build MSI installer]
    msisign[Sign MSI installer]
    uploadmsi[Upload MSI to GHA artifacts.zip]
    
    fetch-->buildmsi-->msisign-->uploadmsi
end
subgraph generate-packages
    fetch2[Fetch MSI + CLI Binaries from GHA artifacts.zip]
    choco[Create chocolatey package]
    zipall[Create octopus-cli-VERSION.zip with all binaries]
    upload3[Upload versioned zip to GHA octopus-cli-VERSION.zip]

    fetch2-->choco-->zipall-->upload3
end
subgraph publish
    fetch3[Fetch octopus-cli-VERSION.zip from GHA]
    octo-push[Push octopus-cli-VERSION.zip to octopus deploy]
    octo-release[Create release in octopus deploy using VERSION]

    fetch3-->octo-push-->octo-release
end

goreleaser --> msi --> generate-packages --> publish
goreleaser --> generate-packages
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


## Homebrew Deployment process

To publish new packages to homebrew, their defined process is that you fork their `Homebrew/core` repo, make a change to the ruby file representing your package, then create a github pull request with that change.

Our process is a bit constrained because RBAC forbids us from merging anything to the git main branch in the OctopusDeploy organization, unless it has been reviewed by a human. Luckily the homebrew core repo is in a different organization, so we can do this:

Pre-requisite: `Homebrew/core` has been cloned into `OctopusDeploy/homebrew-core`
```mermaid
flowchart TD;
    start[New package version N.NN.NN]
    clone[git clone OctopusDeploy/homebrew-core]
    branch[git checkout -b bump-octopus-cli-N.NN.NN]
    update[Update formula ruby file for new package]
    push[git push -u origin bump-octopus-cli-N.NN.NN]
    pr["gh pr create --base Homebrew/core --title 'octopus-cli N.NN.NN'"]
    
    start-->clone-->branch-->update-->push-->pr
```