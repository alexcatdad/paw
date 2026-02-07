# Changelog

## [0.1.1](https://github.com/alexcatdad/paw/compare/v0.1.0...v0.1.1) (2026-02-07)


### Features

* add security check for personal info leaks ([171d16f](https://github.com/alexcatdad/paw/commit/171d16fea2c3c596362c9a2b69f10da2c94a3d9a))
* **audit:** add audit command to CLI ([7c26e12](https://github.com/alexcatdad/paw/commit/7c26e1278df1a2d220db3ae8594074a84bb936da))
* **audit:** add audit types ([e52585e](https://github.com/alexcatdad/paw/commit/e52585ee26967ba4562db2042b8339fef9bd25d2))
* **audit:** add common config patterns ([8953d11](https://github.com/alexcatdad/paw/commit/8953d11b1656dc1446ed3a468ebebe6ff387069c))
* **audit:** add core audit module ([e0e9467](https://github.com/alexcatdad/paw/commit/e0e9467d76803e3e96d09fc2d509e108db306ccf))
* **backup:** add preRollback/postRollback lifecycle hooks ([af1b6cf](https://github.com/alexcatdad/paw/commit/af1b6cf34c83ef0edaaed02169d89075b7980f3e))
* **ci:** add GitHub attestation for binary verification ([810967b](https://github.com/alexcatdad/paw/commit/810967b66298ac9909be7bd5cfe3bb79f64ae595))
* **cli:** add --no-interactive flag for non-TTY environments ([43387ab](https://github.com/alexcatdad/paw/commit/43387ab6c5643c10047d56d458fbd9744bf203f4))
* **docs:** add GitHub Pages demo site with brutalist design ([bc8403d](https://github.com/alexcatdad/paw/commit/bc8403d655a471a9296a3ead5b4697280952f84c))
* **hooks:** add centralized hook runner helper ([80bcd35](https://github.com/alexcatdad/paw/commit/80bcd35b3f126cc83756fb87d2f5da78db65ca08))
* **os:** add matchGlob helper and getHostname for machine-specific configs ([7598ab4](https://github.com/alexcatdad/paw/commit/7598ab48a2f58993dbed5201479fdc4f69e2a78f))
* **prompt:** add interactive conflict resolution prompts ([7314e19](https://github.com/alexcatdad/paw/commit/7314e19f50b58f4a2127e75c8b9865ea20f8beb1))
* **push:** add prePush/postPush lifecycle hooks ([159842a](https://github.com/alexcatdad/paw/commit/159842add4fcafe5c5379f380c04f99e4539dab9))
* **scaffold:** add scaffold command for generating configs ([10e2f3d](https://github.com/alexcatdad/paw/commit/10e2f3dd231561da1554f53f090a00b9fcd23349))
* **symlinks:** integrate interactive conflict resolution ([c2e90cd](https://github.com/alexcatdad/paw/commit/c2e90cdabc086b19c8c8081e44c3d491e8e173fd))
* **symlinks:** support conditional symlinks with hostname/platform matching ([f1eef6f](https://github.com/alexcatdad/paw/commit/f1eef6f80ebee4b3e89b9466b709a1e71cc9b304))
* **sync:** add preSync/postSync lifecycle hooks ([835ef59](https://github.com/alexcatdad/paw/commit/835ef598d531839e6632929f31d74f79f526761d))
* **types:** add full lifecycle hook types (sync, push, update, rollback) ([1ddc760](https://github.com/alexcatdad/paw/commit/1ddc7605a5d3682e1ca5553a4fc9ac896bdb0612))
* **types:** add SymlinkCondition and SymlinkTarget for machine-specific configs ([bbe4440](https://github.com/alexcatdad/paw/commit/bbe444079cd4f65f5db6a767d2cad3972257302d))
* **update:** add binary attestation verification with --skip-verify escape hatch ([d284ebe](https://github.com/alexcatdad/paw/commit/d284ebe655b2859022d6fdc12f616d46d9170a90))
* **update:** add preUpdate/postUpdate lifecycle hooks ([7da50a4](https://github.com/alexcatdad/paw/commit/7da50a41598bddc0b0cfcb11cf1f0a816d9f1985))


### Bug Fixes

* **ci:** resolve security-check.sh false positives ([5f88c2a](https://github.com/alexcatdad/paw/commit/5f88c2a97bab8ec3dc6ca4e69905e13644434121))
* **update:** correctly detect compiled binary ([e3bc7c3](https://github.com/alexcatdad/paw/commit/e3bc7c3e28cb011c5637197a111a7a4c5c25480a))
