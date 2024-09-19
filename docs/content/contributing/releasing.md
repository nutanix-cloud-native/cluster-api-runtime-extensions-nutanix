+++
title = "Releasing"
icon = "fa-solid fa-gift"
+++

## Creating a release PR

This project uses [release-please] to automate changelog updates per release. Due to security restrictions[^1] in the
`nutanix-cloud-native` GitHub organization, the release process is a little more complex than just using the
[release-please-action].

When a release has been cut, a new release PR can be created manually using the `release-please` CLI locally. This needs
to be run by someone with write permissions to the repository.
The new release PR can be only created against `main` or `release/*` branch.
Ensure to checkout `main` or `release/*` branch locally.
Create the `release-please` branch and PR from `main` or `release/*` branch:

```shell
make release-please
```

This will create the branch and release PR. From this point on until a release is ready, the `release-please-action`
will keep the PR up to date (GHA workflows are only not allowed to create the original PR, they can keep the PR up to
date).

## Cutting a release

When a release is ready, the commits in the release PR created above will need to be signed (again, this is a security
requirement). To do this, check out the PR branch locally:

```shell
gh pr checkout <RELEASE_PR_NUMBER>
```

Sign the previous commit:

```bash
git commit --gpg-sign --amend --no-edit
```

If you are releasing a new minor release, then update the `metadata.yaml`s so that the upcoming release version is used
for e.g. local development and e2e tests:

1. Add the new release to the root level `metadata.yaml` release series.
1. Add the new release to the e2e configuration `test/e2e/data/shared/v1beta1-caren/metadata.yaml` release series.
1. Add the next release to the e2e configuration `test/e2e/data/shared/v1beta1-caren/metadata.yaml` (e.g. if release
   `v0.6.0` then add release series for `v0.7`).
1. Update the `caren` provider configuration in `test/e2e/config/caren.yaml` with the new release (replacing the last
   minor release with the new minor release version) and the next minor release configuration (replacing the `v0.x.99`
   configuration).
1. Commit the changed files:

   ```shell
   git add metadata.yaml test/e2e/data/shared/v1beta1-caren/metadata.yaml test/e2e/config/caren.yaml
   git commit --gpg-sign -m 'fixup! release: Update metadata for release'
   ```

And force push:

```shell
git push --force-with-lease
```

The PR will then need the standard 2 reviewers and will then be auto-merged, triggering the release jobs to run and push
relevant artifacts and images.

[^1]: Specifically, GitHub Actions workflows are not allowed to create or approve PRs due to a potential security flaw.
    See [this blog post][cider-sec] for more details, as well as the [Security Hardening for GitHub Actions
    docs][gha-security-hardening].

[release-please]: https://github.com/googleapis/release-please/
[release-please-action]: https://github.com/googleapis/release-please-action
[cider-sec]: https://medium.com/cider-sec/bypassing-required-reviews-using-github-actions-6e1b29135cc7
[gha-security-hardening]: https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions
