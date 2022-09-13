# Changelog

## [0.2.1](https://github.com/OctopusDeploy/cli/compare/v0.2.0...v0.2.1) (2022-09-12)


### Features

* config file support ([#66](https://github.com/OctopusDeploy/cli/issues/66)) ([d911e0c](https://github.com/OctopusDeploy/cli/commit/d911e0caa04477ce677bd7e0652d06de18081c79))
* deploy release ([a2f85f0](https://github.com/OctopusDeploy/cli/commit/a2f85f0b357200d932cb4c0324678702b85c91e3))
* surveyext.DatePicker ([a2f85f0](https://github.com/OctopusDeploy/cli/commit/a2f85f0b357200d932cb4c0324678702b85c91e3))


### Bug Fixes

* Use survey's terminal.NewAnsiStdout to get colors to work in windows cmd.exe ([#68](https://github.com/OctopusDeploy/cli/issues/68)) ([01e02aa](https://github.com/OctopusDeploy/cli/commit/01e02aa39a442cdb896ef12715eadef94b150333)), closes [#67](https://github.com/OctopusDeploy/cli/issues/67)

## [0.2.0](https://github.com/OctopusDeploy/cli/compare/v0.1.0...v0.2.0) (2022-08-26)


### Features

* `release create`
* `release list`
* `release delete`
* `account <type> list commands` ([#41](https://github.com/OctopusDeploy/cli/issues/41)) ([5354118](https://github.com/OctopusDeploy/cli/commit/5354118066be9c31c83cd617f3ea50ff31f8b7ec))
* `account create` ([#47](https://github.com/OctopusDeploy/cli/issues/47)) ([11d67d1](https://github.com/OctopusDeploy/cli/commit/11d67d101217a2935a54bdb6534899c293d2a7a5))
* allow selection of space on command line ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* CLI now prompts to select a space in interactive mode, or auto-selects if there is only one visible space. ([1f57272](https://github.com/OctopusDeploy/cli/commit/1f572720e5f797c45efe256cee919a2a37acf698))
* compatibility with legacy flags from the .NET CLI ([c439c2e](https://github.com/OctopusDeploy/cli/commit/c439c2e954f44b04b3975622d918d2bf3b6054bf))
* updated MultiSelectMap to return via generics ([d7d23d0](https://github.com/OctopusDeploy/cli/commit/d7d23d0b1cc4cc2d78a490a901c2e47d3fdf85f5))

### Bug Fixes

* --no-prompt and the CI environment check wasn't respected if the CLI wanted to prompt for the space name ([c75cf69](https://github.com/OctopusDeploy/cli/commit/c75cf69898504bcd074cda4a4bb153b096cbcff2))
* Activity spinner was showing over the top of the space selection prompt in interactive mode ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))
* detection of terminal width when outputting tables ([10e1a9b](https://github.com/OctopusDeploy/cli/commit/10e1a9beec011249806e8506a3d44646f5fb8809))
* flag alias for --output-format and --outputFormat didn't work ([0d42bc0](https://github.com/OctopusDeploy/cli/commit/0d42bc037572be9ce69cc135aa1dd51576666b36))

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
