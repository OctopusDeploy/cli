# create-homebrew-pr.ps1

$origin="borland/homebrew-core" # should be Homewbrew/homebrew-core for production
$packageVersion="0.2.0"
$packageUrl="https://github.com/OctopusDeploy/cli/releases/download/v${packageVersion}/octopus_${packageVersion}_Darwin_arm64.tar.gz"
$formulaFile="octopus-cli.rb"

Invoke-WebRequest $packageUrl -outfile pkg.tgz
$sha256=(Get-FileHash pkg.tgz -a sha256).Hash.ToLowerInvariant()
rm pkg.tgz

# git clone --depth 1 $origin our-homebrew-core
# cd our-homebrew-core/Formula

# git checkout -b bump-octopus-cli-$packageVersion

((Get-Content $formulaFile) `
    -replace "version `".*`"", "version `"$packageVersion`"" `
    -replace "url `".*`"", "url `"$packageUrl`"" `
    -replace "sha256 `".*`"", "sha256 `"$sha256`"") `
    | Set-Content $formulaFile

# git commit -a -m "octopus-cli $packageVersion"

# gh pr create --base $origin --title "octopus-cli $packageVersion"






# packageVersion="$(get_octopusvariable 'Octopus.Action.Package[cli].PackageVersion')"
# extractedPath="$(get_octopusvariable 'Octopus.Action.Package[cli].ExtractedPath')"

# username="$(get_octopusvariable 'Publish:HomeBrew:Username')"
# email="$(get_octopusvariable 'Publish:HomeBrew:UserEmail')"
# personalAccessToken="$(get_octopusvariable 'Publish:HomeBrew:ApiKey')"

# orgName="OctopusDeploy"
# repoName="$(get_octopusvariable 'Publish:HomeBrew:RepoName')"
