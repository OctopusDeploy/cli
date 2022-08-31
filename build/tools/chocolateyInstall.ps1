$ErrorActionPreference = 'Stop'

$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$logMsi = Join-Path -Path $env:TEMP -ChildPath ("{0}-{1}-MsiInstall.log" -f $env:ChocolateyPackageName, $env:chocolateyPackageVersion)

$packageArgs = @{
    packageName    = $env:ChocolateyPackageName
    fileType       = 'MSI'
    silentArgs     = "/qn /norestart /l*v `"$logMsi`""
    file64         = Join-Path -Path $toolsDir -ChildPath "octopus_$($env:ChocolateyPackageVersion)_windows_x86_64.msi"
}

Install-ChocolateyInstallPackage @packageArgs