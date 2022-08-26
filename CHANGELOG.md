# Changelog

## [0.2.0](https://github.com/OctopusDeploy/cli/compare/v0.1.0...v0.2.0) (2022-08-26)


### Features

* --no-prompt flag on the command line to force automation mode ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* --release-notes and --release-notes-file ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))
* account <type> list commands ([#41](https://github.com/OctopusDeploy/cli/issues/41)) ([5354118](https://github.com/OctopusDeploy/cli/commit/5354118066be9c31c83cd617f3ea50ff31f8b7ec))
* account create ([#47](https://github.com/OctopusDeploy/cli/issues/47)) ([11d67d1](https://github.com/OctopusDeploy/cli/commit/11d67d101217a2935a54bdb6534899c293d2a7a5))
* allow selection of space on command line ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* basic release creation specifying project, channel, version implemented ([c75cf69](https://github.com/OctopusDeploy/cli/commit/c75cf69898504bcd074cda4a4bb153b096cbcff2))
* basic support for release creation ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* CLI now prompts to select a space in interactive mode, or auto-selects if there is only one visible space. ([1f57272](https://github.com/OctopusDeploy/cli/commit/1f572720e5f797c45efe256cee919a2a37acf698))
* compatibility with legacy flags from the .NET CLI ([c439c2e](https://github.com/OctopusDeploy/cli/commit/c439c2e954f44b04b3975622d918d2bf3b6054bf))
* create account links and automation cmd generation ([9aa6f12](https://github.com/OctopusDeploy/cli/commit/9aa6f128e4cc15f06aa6f7cd3cfb7d903dbc7fb5))
* create AWS account ([#45](https://github.com/OctopusDeploy/cli/issues/45)) ([9dd0d64](https://github.com/OctopusDeploy/cli/commit/9dd0d64755cef15dcc01a8c1a9df7b0605d3b1d2))
* custom help page ([804a618](https://github.com/OctopusDeploy/cli/commit/804a6180d8151f971e28eae61c2b3253bd60b326))
* If a release version is determined by a donor package, lock that in as the base version and only prompt for version metadata ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* output web url after release create ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))
* package help text, package table reset ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* package version support in create release ([fff48d3](https://github.com/OctopusDeploy/cli/commit/fff48d3a3e77e738dfb068849beea0c795ba2345))
* release create ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* release create gains outputformat json and basic ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* release create outputs web URL's all the time ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* release create supports --git-commit and --git-ref on the command line ([68de52b](https://github.com/OctopusDeploy/cli/commit/68de52b6c90a2996044658fa4249a5bf70bdb20a))
* release create supports unresolved packages ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))
* release delete ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* release list ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))
* release list and release create ([ce93d7e](https://github.com/OctopusDeploy/cli/commit/ce93d7edc32b7d9924268328662193cede2a309e))
* remove --package-prerelease; we've decided not to support it in favour of channels ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))
* support for --package-prerelease ([c439c2e](https://github.com/OctopusDeploy/cli/commit/c439c2e954f44b04b3975622d918d2bf3b6054bf))
* undo support for package version query loop ([c439c2e](https://github.com/OctopusDeploy/cli/commit/c439c2e954f44b04b3975622d918d2bf3b6054bf))
* updated MultiSelectMap to return via generics ([d7d23d0](https://github.com/OctopusDeploy/cli/commit/d7d23d0b1cc4cc2d78a490a901c2e47d3fdf85f5))
* ux tweaks for package version table ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))
* validation of package versions ("dog" is no longer a valid package version) ([2e69b9a](https://github.com/OctopusDeploy/cli/commit/2e69b9a95dfe5e041976bda69b54192ab466aec1))


### Bug Fixes

* --no-prompt and the CI environment check wasn't respected if the CLI wanted to prompt for the space name ([c75cf69](https://github.com/OctopusDeploy/cli/commit/c75cf69898504bcd074cda4a4bb153b096cbcff2))
* Activity spinner was showing over the top of the space selection prompt in interactive mode ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* create account ([#46](https://github.com/OctopusDeploy/cli/issues/46)) ([5ba7fd5](https://github.com/OctopusDeploy/cli/commit/5ba7fd5c66a5a3bbac06ad12a61300e215add2d5))
* detection of terminal width when outputting tables ([10e1a9b](https://github.com/OctopusDeploy/cli/commit/10e1a9beec011249806e8506a3d44646f5fb8809))
* flag alias for --output-format and --outputFormat didn't work ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* use gitRef canonical name when addressing version controlled repositories ([f586b84](https://github.com/OctopusDeploy/cli/commit/f586b847d2b735df8fbbd323c4950fd0989dc620))

## [0.1.0](https://github.com/OctopusDeploy/cli/compare/v0.0.3...v0.1.0) (2022-07-24)


### Features

* Add os env variable validation ([#19](https://github.com/OctopusDeploy/cli/issues/19)) ([9a577d1](https://github.com/OctopusDeploy/cli/commit/9a577d17f0fdfc365ffcd0b35f4a92c7d1571428))

## [0.0.3](https://github.com/OctopusDeploy/cli/compare/v0.0.2...v0.0.3) (2022-07-22)


### Bug Fixes

* signing ([#22](https://github.com/OctopusDeploy/cli/issues/22)) ([908484b](https://github.com/OctopusDeploy/cli/commit/908484b9561423528e1a3ffcc10f308a02c0b1e7))

## [0.0.2](https://github.com/OctopusDeploy/cli/compare/v0.0.1...v0.0.2) (2022-07-22)


### Bug Fixes

* package signing ([#20](https://github.com/OctopusDeploy/cli/issues/20)) ([d9ea487](https://github.com/OctopusDeploy/cli/commit/d9ea487220e4ba7a8dbeff98ace9b7680dd67679))

## [0.0.1](https://github.com/OctopusDeploy/cli/compare/v0.0.1...v0.0.1) (2022-07-22)


### Features

* init release ([4628209](https://github.com/OctopusDeploy/cli/commit/4628209371341bb7ac93d2ff4f590f09b7633816))


### Miscellaneous Chores

* change release please to simple mode ([#14](https://github.com/OctopusDeploy/cli/issues/14)) ([9615fa1](https://github.com/OctopusDeploy/cli/commit/9615fa19d45e9c4b4e73a1a7c14ed0072614d1b1))

## [0.0.1](https://github.com/OctopusDeploy/cli/compare/v1.2.0...v0.0.1) (2022-07-22)


### Features

* init release ([4628209](https://github.com/OctopusDeploy/cli/commit/4628209371341bb7ac93d2ff4f590f09b7633816))


### Miscellaneous Chores

* change release please to simple mode ([#14](https://github.com/OctopusDeploy/cli/issues/14)) ([9615fa1](https://github.com/OctopusDeploy/cli/commit/9615fa19d45e9c4b4e73a1a7c14ed0072614d1b1))
