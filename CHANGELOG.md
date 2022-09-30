# Changelog

## [0.3.6](https://github.com/OctopusDeploy/cli/compare/v0.3.5...v0.3.6) (2022-09-30)


### Bug Fixes

* Add changed file before calling commit ([#101](https://github.com/OctopusDeploy/cli/issues/101)) ([552222a](https://github.com/OctopusDeploy/cli/commit/552222a16069a794e34892208f4146e2ed48633a))

## [0.3.5](https://github.com/OctopusDeploy/cli/compare/v0.3.4...v0.3.5) (2022-09-30)


### Bug Fixes

* Refactor git command code to powershell instead of bash ([#99](https://github.com/OctopusDeploy/cli/issues/99)) ([3d5a1a8](https://github.com/OctopusDeploy/cli/commit/3d5a1a8b75bd841fed21e163ff94eb0992b6167f))

## [0.3.4](https://github.com/OctopusDeploy/cli/compare/v0.3.3...v0.3.4) (2022-09-30)


### Bug Fixes

* PR creation is automated in the homebrew-taps repo so no need to create it here ([#97](https://github.com/OctopusDeploy/cli/issues/97)) ([ee03780](https://github.com/OctopusDeploy/cli/commit/ee03780ad54c915d6a78fbd6d0815d88cde2f232))

## [0.3.3](https://github.com/OctopusDeploy/cli/compare/v0.3.2...v0.3.3) (2022-09-30)


### Bug Fixes

* Fix issue when creating PR to update homebrew formula ([#95](https://github.com/OctopusDeploy/cli/issues/95)) ([03929eb](https://github.com/OctopusDeploy/cli/commit/03929ebd8a4f33c65ea8d0faf0e92449a0f4e74e))

## [0.3.2](https://github.com/OctopusDeploy/cli/compare/v0.3.1...v0.3.2) (2022-09-30)


### Bug Fixes

* Fix bug in deployment process causing homebrew update to fail ([#93](https://github.com/OctopusDeploy/cli/issues/93)) ([6c5ee67](https://github.com/OctopusDeploy/cli/commit/6c5ee678f1037415c4cbb7ab2605f9b3b20b780e))

## [0.3.1](https://github.com/OctopusDeploy/cli/compare/v0.3.0...v0.3.1) (2022-09-30)


### Bug Fixes

* Fix invalid syntax in script for pushing the updated homebrew formula ([#90](https://github.com/OctopusDeploy/cli/issues/90)) ([75e39a8](https://github.com/OctopusDeploy/cli/commit/75e39a84b8641f361e8fabaaa203426029b571ca))
* Sign rpm/deb packages as part of the creation process in goreleaser ([#92](https://github.com/OctopusDeploy/cli/issues/92)) ([b30498f](https://github.com/OctopusDeploy/cli/commit/b30498fb54e140f85056fc5dc271182cd584dd6f))

## [0.3.0](https://github.com/OctopusDeploy/cli/compare/v0.2.5...v0.3.0) (2022-09-29)


### Features

* accounts slugs ([#87](https://github.com/OctopusDeploy/cli/issues/87)) ([724f365](https://github.com/OctopusDeploy/cli/commit/724f365ba5bc908b24e69c21653c870dca345c68))
* package list ([28090f3](https://github.com/OctopusDeploy/cli/commit/28090f32bd907f80fdf0e455f35982f3a546395d))
* package upload/push ([28090f3](https://github.com/OctopusDeploy/cli/commit/28090f32bd907f80fdf0e455f35982f3a546395d))
* package versions ([28090f3](https://github.com/OctopusDeploy/cli/commit/28090f32bd907f80fdf0e455f35982f3a546395d))


### Bug Fixes

* octopus config set in interactive mode would fail for anything that wasn't already in the config file ([#83](https://github.com/OctopusDeploy/cli/issues/83)) ([0b43d8b](https://github.com/OctopusDeploy/cli/commit/0b43d8b1155e09955facb2b39024ea4aa7e32f2e))
* Update name of msi and add missing log argument to MSI call for Chocolatey ([7504b7d](https://github.com/OctopusDeploy/cli/commit/7504b7dd445146fdf2e8f35e1858135f2a56b2f0))

## [0.2.5](https://github.com/OctopusDeploy/cli/compare/v0.2.4...v0.2.5) (2022-09-20)


### Bug Fixes

* cli[#71](https://github.com/OctopusDeploy/cli/issues/71) - config system was leaking env vars into the config file when it shouldn't have  ([#80](https://github.com/OctopusDeploy/cli/issues/80)) ([e8edb32](https://github.com/OctopusDeploy/cli/commit/e8edb32247e4baa438e11238b61fddf7cb0ae595))

## [0.2.4](https://github.com/OctopusDeploy/cli/compare/v0.2.3...v0.2.4) (2022-09-16)


### Bug Fixes

* iron out the deployment process ([#78](https://github.com/OctopusDeploy/cli/issues/78)) ([fd59f84](https://github.com/OctopusDeploy/cli/commit/fd59f84b1be6b4c329583149577d540736646720))

## [0.2.3](https://github.com/OctopusDeploy/cli/compare/v0.2.2...v0.2.3) (2022-09-15)


### Bug Fixes

* releasenotes pickup in build flow, update go-octopusdeploy to 2.4.1 ([#76](https://github.com/OctopusDeploy/cli/issues/76)) ([4dc9550](https://github.com/OctopusDeploy/cli/commit/4dc955017ebbb12031830da37ab2d3ce3f3e3c78))

## [0.2.2](https://github.com/OctopusDeploy/cli/compare/v0.2.1...v0.2.2) (2022-09-13)


### Bug Fixes

* expand goreleaser workflow to build MSI and chocolatey packages ([1d86a77](https://github.com/OctopusDeploy/cli/commit/1d86a77ead003a199b3ee987c044dc45ae21e9ae))

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
