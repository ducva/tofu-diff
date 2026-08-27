# Bundle update log

## 2026-08-27

* **History reset**: Re-grounded the bundle in the current root working tree after Git history was cleared, and removed obsolete repository revision metadata.
* **Source cleanup**: Removed the requirements/SPDD concept and navigation because those source directories are no longer present in the reset repository.
* **Repository identity**: Updated the documented Go module path to match the public `github.com/ducva/tofu-diff` repository and installation command.
* **Release automation**: Documented the `main` push workflow that tests the CLI and publishes cross-platform GitHub release archives under automatic `v0.0.<run-number>` tags.
* **Initialization**: Created the initial tofu-diff OKF v0.2 bundle.
* **Creation**: Added architecture, plan-processing, interfaces, and conventions concepts.
