# Changelog

## [1.7.0](https://github.com/OctopusDeploy/cli/compare/v1.6.2...v1.7.0) (2023-10-31)


### Features

* Update fields included in JSON output for workers list ([#284](https://github.com/OctopusDeploy/cli/issues/284)) ([9bb3d43](https://github.com/OctopusDeploy/cli/commit/9bb3d43ba9eddf39d73887bb021683b953f15776))

## [1.6.2](https://github.com/OctopusDeploy/cli/compare/v1.6.1...v1.6.2) (2023-09-14)


### Bug Fixes

* octopus deploy process for homebrew ([3c82836](https://github.com/OctopusDeploy/cli/commit/3c828368cee0b01844d46eaa6076469825f73e53))

## [1.6.1](https://github.com/OctopusDeploy/cli/compare/v1.6.0...v1.6.1) (2023-09-14)


### Bug Fixes

* homebrew deployments ([#273](https://github.com/OctopusDeploy/cli/issues/273)) ([4d3845a](https://github.com/OctopusDeploy/cli/commit/4d3845ad5bbc1529f77161d028ce4537626c5053))

## [1.6.0](https://github.com/OctopusDeploy/cli/compare/v1.5.1...v1.6.0) (2023-09-08)


### Features

* Adds support for authenticating with access token ([#270](https://github.com/OctopusDeploy/cli/issues/270)) ([75a8ff7](https://github.com/OctopusDeploy/cli/commit/75a8ff7b622f6ed9a353c83a4d3afa7dc0f93b9f))

## [1.5.1](https://github.com/OctopusDeploy/cli/compare/v1.5.0...v1.5.1) (2023-08-10)


### Bug Fixes

* prompt checks for git credentials storage was case-sensitive ([#265](https://github.com/OctopusDeploy/cli/issues/265)) ([c9a5817](https://github.com/OctopusDeploy/cli/commit/c9a5817dfb8e0875a0b70e0ae6243f5012eaedef))

## [1.5.0](https://github.com/OctopusDeploy/cli/compare/v1.4.0...v1.5.0) (2023-07-25)


### Features

* project branch create ([101b09e](https://github.com/OctopusDeploy/cli/commit/101b09e409c2885beb19ae9e66b88a977f0c985b))
* project branch list ([101b09e](https://github.com/OctopusDeploy/cli/commit/101b09e409c2885beb19ae9e66b88a977f0c985b))


### Bug Fixes

* admin login property on k8s target was wrong type ([#249](https://github.com/OctopusDeploy/cli/issues/249)) ([fbe8e18](https://github.com/OctopusDeploy/cli/commit/fbe8e18d041cf416fc0013451f416363eb0a83e5))
* support Config-as-code project variables ([101b09e](https://github.com/OctopusDeploy/cli/commit/101b09e409c2885beb19ae9e66b88a977f0c985b))

## [1.4.0](https://github.com/OctopusDeploy/cli/compare/v1.3.0...v1.4.0) (2023-06-08)


### Features

* project clone ([#244](https://github.com/OctopusDeploy/cli/issues/244)) ([f6a0ba5](https://github.com/OctopusDeploy/cli/commit/f6a0ba5d7a1f55f78ea6c6eb2fb8ecfa38bf1c4a))


### Bug Fixes

* sort the possible variables before displaying ([#240](https://github.com/OctopusDeploy/cli/issues/240)) ([61f5073](https://github.com/OctopusDeploy/cli/commit/61f5073d0d2b247ceddfc10902072357fdb0d685))

## [1.3.0](https://github.com/OctopusDeploy/cli/compare/v1.2.2...v1.3.0) (2023-05-23)


### Features

* tenant variable list ([69bb317](https://github.com/OctopusDeploy/cli/commit/69bb3177441b7d4d9e2928e564a61e36343e0a7c))
* tenant variable update ([69bb317](https://github.com/OctopusDeploy/cli/commit/69bb3177441b7d4d9e2928e564a61e36343e0a7c))

## [1.2.2](https://github.com/OctopusDeploy/cli/compare/v1.2.1...v1.2.2) (2023-05-21)


### Bug Fixes

* fixed incorrect example ([#234](https://github.com/OctopusDeploy/cli/issues/234)) ([969ef1b](https://github.com/OctopusDeploy/cli/commit/969ef1b17cad755c8f7a5087c358e97cac8bad86))

## [1.2.1](https://github.com/OctopusDeploy/cli/compare/v1.2.0...v1.2.1) (2023-05-12)


### Bug Fixes

* added support for project slugs to release commands ([#230](https://github.com/OctopusDeploy/cli/issues/230)) ([851631a](https://github.com/OctopusDeploy/cli/commit/851631a0086549fc12528b72f48be9952d5a84af))

## [1.2.0](https://github.com/OctopusDeploy/cli/compare/v1.1.0...v1.2.0) (2023-01-31)


### Features

* project variables exclude ([9e0c0b8](https://github.com/OctopusDeploy/cli/commit/9e0c0b8584b80c5bfa87915b39c5991ebe61ff51))
* project variables include ([9e0c0b8](https://github.com/OctopusDeploy/cli/commit/9e0c0b8584b80c5bfa87915b39c5991ebe61ff51))

## [1.1.0](https://github.com/OctopusDeploy/cli/compare/v1.0.0...v1.1.0) (2023-01-25)


### Features

* project variables create ([#198](https://github.com/OctopusDeploy/cli/issues/198)) ([faa3b4c](https://github.com/OctopusDeploy/cli/commit/faa3b4c46dad0167859d3e053dec9229832b9159))
* project variables delete ([faa3b4c](https://github.com/OctopusDeploy/cli/commit/faa3b4c46dad0167859d3e053dec9229832b9159))
* project variables list ([faa3b4c](https://github.com/OctopusDeploy/cli/commit/faa3b4c46dad0167859d3e053dec9229832b9159))
* project variables update ([faa3b4c](https://github.com/OctopusDeploy/cli/commit/faa3b4c46dad0167859d3e053dec9229832b9159))
* project variables view ([faa3b4c](https://github.com/OctopusDeploy/cli/commit/faa3b4c46dad0167859d3e053dec9229832b9159))


### Bug Fixes

* empty list errors ([#208](https://github.com/OctopusDeploy/cli/issues/208)) ([74cbfe2](https://github.com/OctopusDeploy/cli/commit/74cbfe2c98c2ce26153fe4c8521b0292c076acb7))
* link convert options to create when in non-interactive mode ([#213](https://github.com/OctopusDeploy/cli/issues/213)) ([adccfd9](https://github.com/OctopusDeploy/cli/commit/adccfd9f19ae8be0332f05d067f536f89cef4e91))

## [1.0.0](https://github.com/OctopusDeploy/cli/compare/v0.10.2...v1.0.0) (2023-01-05)


### ⚠ BREAKING CHANGES

* package

### Features

* package ([088df06](https://github.com/OctopusDeploy/cli/commit/088df0673c2bf88928f2f5ce86dff46533cb0dd7))


### Bug Fixes

* null ref exception in  `account create` command ([#204](https://github.com/OctopusDeploy/cli/issues/204)) ([204f706](https://github.com/OctopusDeploy/cli/commit/204f706b48bb606fa52e7ef5223845775d344ec6))
* remove project convert support for default protected branch  ([#199](https://github.com/OctopusDeploy/cli/issues/199)) ([05ee50a](https://github.com/OctopusDeploy/cli/commit/05ee50a9d8a7e04ea7afbf1858c3f2d5bfd909a1))

## [0.10.2](https://github.com/OctopusDeploy/cli/compare/v0.10.1...v0.10.2) (2022-12-23)


### Bug Fixes

* Fixed go releaser setup to fix release notes ([#196](https://github.com/OctopusDeploy/cli/issues/196)) ([7d5f720](https://github.com/OctopusDeploy/cli/commit/7d5f72020317286479d2557a00bb5e8d56db06b8))

## [0.10.1](https://github.com/OctopusDeploy/cli/compare/v0.10.0...v0.10.1) (2022-12-23)


### Bug Fixes

* Fixes for Go Releaser, moving it to Ocotpus' v3 GitHub Actions ([#194](https://github.com/OctopusDeploy/cli/issues/194)) ([77f023b](https://github.com/OctopusDeploy/cli/commit/77f023b31f0c4b50f6d213b97a7ff3f3eea364dc))

## [0.10.0](https://github.com/OctopusDeploy/cli/compare/v0.9.0...v0.10.0) (2022-12-23)


### Features

* Fixed the OCTOPUS_URL environment variable name ([#191](https://github.com/OctopusDeploy/cli/issues/191)) ([1aa1a98](https://github.com/OctopusDeploy/cli/commit/1aa1a98170b82fb536d67541d913fb825f9c075f))


### Bug Fixes

* typo in workerpool message ([#193](https://github.com/OctopusDeploy/cli/issues/193)) ([baf2a46](https://github.com/OctopusDeploy/cli/commit/baf2a46d70582b101f952703ed3605e4210dc3a6))

## [0.9.0](https://github.com/OctopusDeploy/cli/compare/v0.8.1...v0.9.0) (2022-12-21)


### Features

* clone tenant ([#184](https://github.com/OctopusDeploy/cli/issues/184)) ([9bc3b6d](https://github.com/OctopusDeploy/cli/commit/9bc3b6d702f69b9c2359d3cf65ade0fa241c0689))

## [0.8.1](https://github.com/OctopusDeploy/cli/compare/v0.8.0...v0.8.1) (2022-12-19)


### Bug Fixes

* updated dependencies ([d009065](https://github.com/OctopusDeploy/cli/commit/d009065b22f48d922f02126e6cf908fe8372bd40))

## [0.8.0](https://github.com/OctopusDeploy/cli/compare/v0.7.1...v0.8.0) (2022-12-19)


### Features

* kubernetes deployment target ([#178](https://github.com/OctopusDeploy/cli/issues/178)) ([6e25759](https://github.com/OctopusDeploy/cli/commit/6e257593b4f08c354249a9faa4ba9549c2136787))

## [0.7.1](https://github.com/OctopusDeploy/cli/compare/v0.7.0...v0.7.1) (2022-12-16)


### Bug Fixes

* expand tenant view output ([#174](https://github.com/OctopusDeploy/cli/issues/174)) ([7e3efce](https://github.com/OctopusDeploy/cli/commit/7e3efce9f7bc5acf8a24a906404b3c7b32209c58))
* extended release output ([#179](https://github.com/OctopusDeploy/cli/issues/179)) ([80e1200](https://github.com/OctopusDeploy/cli/commit/80e120078c807baa451b887a6e736acdbfe20dce))

## [0.7.0](https://github.com/OctopusDeploy/cli/compare/v0.6.0...v0.7.0) (2022-12-13)


### Features

* `worker-pool dynamic view` ([b2cd68c](https://github.com/OctopusDeploy/cli/commit/b2cd68cae80cc913875674c0dbc84dd9ec54c84b))
* `worker-pool static view` ([b2cd68c](https://github.com/OctopusDeploy/cli/commit/b2cd68cae80cc913875674c0dbc84dd9ec54c84b))
* basic `user list` and `user delete` ([74fdc7e](https://github.com/OctopusDeploy/cli/commit/74fdc7e0a811dcc6444d2fd48df3cbb661e87503))
* convert existing project to Config As Code ([2788964](https://github.com/OctopusDeploy/cli/commit/2788964e8f5aa799d16b6259118adec7f4228f7a))
* worker-pool delete ([#161](https://github.com/OctopusDeploy/cli/issues/161)) ([bb11b10](https://github.com/OctopusDeploy/cli/commit/bb11b105fea2d6253c67077e51a2a2657be6afd7))
* worker-pool list ([#158](https://github.com/OctopusDeploy/cli/issues/158)) ([b2cd68c](https://github.com/OctopusDeploy/cli/commit/b2cd68cae80cc913875674c0dbc84dd9ec54c84b))
* workerpool static create ([#162](https://github.com/OctopusDeploy/cli/issues/162)) ([3f3b66c](https://github.com/OctopusDeploy/cli/commit/3f3b66cf8ca59e5707ab85f5dddbd4b171a00f70))


### Bug Fixes

* skip prompt if only a single channel ([#172](https://github.com/OctopusDeploy/cli/issues/172)) ([01e3cbb](https://github.com/OctopusDeploy/cli/commit/01e3cbb7f1b15eafe5793e5bd2920d31774b4537))
* validate environments for tenanted deployments ([#168](https://github.com/OctopusDeploy/cli/issues/168)) ([c4f7cb9](https://github.com/OctopusDeploy/cli/commit/c4f7cb904723ee198549eb21e62c93575514e215))
* version shortcut on release create ([#171](https://github.com/OctopusDeploy/cli/issues/171)) ([9183324](https://github.com/OctopusDeploy/cli/commit/918332406f2efed3014edbdad64f60727e04d704))

## [0.6.0](https://github.com/OctopusDeploy/cli/compare/v0.5.0...v0.6.0) (2022-11-23)


### Features

* create azure web app target ([20d66b3](https://github.com/OctopusDeploy/cli/commit/20d66b32518685d49aff1ec9558c39d370ab4831))
* create cloud region deployment target ([#137](https://github.com/OctopusDeploy/cli/issues/137)) ([672fd7f](https://github.com/OctopusDeploy/cli/commit/672fd7f027d6fab1a122e107f0f00943acd5ce12))
* create listening tentacle ([#136](https://github.com/OctopusDeploy/cli/issues/136)) ([51ff9ec](https://github.com/OctopusDeploy/cli/commit/51ff9eccf8e9b51d3a92b5293656dc3ab2721935))
* create ssh deployment target ([a169d2e](https://github.com/OctopusDeploy/cli/commit/a169d2e0e898bfcf2cf2e749c002aa9ba9779269))
* deployment-target azure-web-app view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target cloud-region view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target listening-tentacle view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target polling-tentacle view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target ssh view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* deployment-target view ([fc639b4](https://github.com/OctopusDeploy/cli/commit/fc639b4b5d3a82538f26e0d90ac47939e8d3c200))
* target delete ([0d81e00](https://github.com/OctopusDeploy/cli/commit/0d81e003a7860fb565cd1c3db1f2879f96d4619c))
* target list  ([4ab66c2](https://github.com/OctopusDeploy/cli/commit/4ab66c2f8234f1e8cf77005a173b50287705c65a))
* tenant create, delete, tag ([8f70286](https://github.com/OctopusDeploy/cli/commit/8f702868e507c02a95fa26e89b02410fd25e0a24))
* tenant view ([7b2f7bd](https://github.com/OctopusDeploy/cli/commit/7b2f7bda5773ec54160f82ed6a6dfc8774ed2cac))
* worker delete ([#147](https://github.com/OctopusDeploy/cli/issues/147)) ([11c838c](https://github.com/OctopusDeploy/cli/commit/11c838cea45c4a02d7df7d092f81ec0b70a99bf0))
* worker list  ([b075cab](https://github.com/OctopusDeploy/cli/commit/b075cab4a1e6b03a1dce90af031aa8642045afb4))
* worker listening-tentacle create ([85f95db](https://github.com/OctopusDeploy/cli/commit/85f95dba761a40955e63ad65269a98081aaa5f6b))
* worker listening-tentacle create ([85f95db](https://github.com/OctopusDeploy/cli/commit/85f95dba761a40955e63ad65269a98081aaa5f6b))
* worker listening-tentacle list ([b075cab](https://github.com/OctopusDeploy/cli/commit/b075cab4a1e6b03a1dce90af031aa8642045afb4))
* worker listening-tentacle view ([c251e1c](https://github.com/OctopusDeploy/cli/commit/c251e1c192ec2106de165ab4e26986d77d8d5422))
* worker polling-tentacle list ([b075cab](https://github.com/OctopusDeploy/cli/commit/b075cab4a1e6b03a1dce90af031aa8642045afb4))
* worker polling-tentacle view ([c251e1c](https://github.com/OctopusDeploy/cli/commit/c251e1c192ec2106de165ab4e26986d77d8d5422))
* worker ssh create ([85f95db](https://github.com/OctopusDeploy/cli/commit/85f95dba761a40955e63ad65269a98081aaa5f6b))
* worker ssh list ([b075cab](https://github.com/OctopusDeploy/cli/commit/b075cab4a1e6b03a1dce90af031aa8642045afb4))
* worker ssh view ([c251e1c](https://github.com/OctopusDeploy/cli/commit/c251e1c192ec2106de165ab4e26986d77d8d5422))
* worker view ([c251e1c](https://github.com/OctopusDeploy/cli/commit/c251e1c192ec2106de165ab4e26986d77d8d5422))
* worker view ([#148](https://github.com/OctopusDeploy/cli/issues/148)) ([c251e1c](https://github.com/OctopusDeploy/cli/commit/c251e1c192ec2106de165ab4e26986d77d8d5422))


### Bug Fixes

* deployment-target list alias ([4bb2086](https://github.com/OctopusDeploy/cli/commit/4bb2086468e3931dde7156dce0bfdc9f259aaa8b))

## [0.5.0](https://github.com/OctopusDeploy/cli/compare/v0.4.0...v0.5.0) (2022-10-31)


### Features

* curl install ([e54dc3b](https://github.com/OctopusDeploy/cli/commit/e54dc3b98cb1cc79831bb5cbeb9b6d60cedb59a0))
* new selector ([564dcff](https://github.com/OctopusDeploy/cli/commit/564dcffae85d9eba1e64f7b0f59ebf3ab24e79a0))
* octopus logo on root help page ([1953205](https://github.com/OctopusDeploy/cli/commit/195320517d7879dabf3cf46f0692679c3d613338))
* project connect ([#122](https://github.com/OctopusDeploy/cli/issues/122)) ([017b958](https://github.com/OctopusDeploy/cli/commit/017b95862e220966dfce48823eb98a3b6abb8f74))
* project create ([#115](https://github.com/OctopusDeploy/cli/issues/115)) ([0a2e409](https://github.com/OctopusDeploy/cli/commit/0a2e40927a93344b76278e02db8dd18e0034f7d0))
* project delete ([#112](https://github.com/OctopusDeploy/cli/issues/112)) ([38039c3](https://github.com/OctopusDeploy/cli/commit/38039c33a7bb1250deac95092f347bc1f8bb6e13))
* project disconnect ([75ebb09](https://github.com/OctopusDeploy/cli/commit/75ebb09ea18594553c3c205239ff63ac61ba4198))
* project-group delete ([#126](https://github.com/OctopusDeploy/cli/issues/126)) ([18b36f6](https://github.com/OctopusDeploy/cli/commit/18b36f680e56a9d91d31658b208382da57f2d2c2))
* project-group list ([#125](https://github.com/OctopusDeploy/cli/issues/125)) ([d6e7f9f](https://github.com/OctopusDeploy/cli/commit/d6e7f9f40237610ebfd79954ce7e1aca424a5202))
* project-group view ([#127](https://github.com/OctopusDeploy/cli/issues/127)) ([ff00390](https://github.com/OctopusDeploy/cli/commit/ff00390e2ce2ba82f366914d493da42155abc90c))
* task wait ([#124](https://github.com/OctopusDeploy/cli/issues/124)) ([a4f925a](https://github.com/OctopusDeploy/cli/commit/a4f925a212fb398188de01a5eb5d9566b2f2e673))
* tenant connect ([#119](https://github.com/OctopusDeploy/cli/issues/119)) ([abc1b31](https://github.com/OctopusDeploy/cli/commit/abc1b31262d8a5a3b9677473a76e9feae620111a))
* tenant disconnect ([75ebb09](https://github.com/OctopusDeploy/cli/commit/75ebb09ea18594553c3c205239ff63ac61ba4198))
* tenant list ([#118](https://github.com/OctopusDeploy/cli/issues/118)) ([4b9c854](https://github.com/OctopusDeploy/cli/commit/4b9c8540842fb266b9d7ed7bf91898e023771a9d))

## [0.4.0](https://github.com/OctopusDeploy/cli/compare/v0.3.6...v0.4.0) (2022-10-10)


### Features

* add project list command ([d82273c](https://github.com/OctopusDeploy/cli/commit/d82273c4718e2bc71e8d62a85d85e26a05768191))
* add project view command ([bb77af9](https://github.com/OctopusDeploy/cli/commit/bb77af98cf6777925be45b51a22ee4b3e866c8ec))
* config list ([00942c3](https://github.com/OctopusDeploy/cli/commit/00942c33509c9f10ee71023d72cddf40efb2b0d2))
* runbook run and list ([70578df](https://github.com/OctopusDeploy/cli/commit/70578df0946a2eaa0414f3abb8a213cc99b9b916))

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
