# Terraform Provider for WarpStream

_This repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework)._

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.20

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.


Create `~/.terraformrc` in order to [Prepare Terraform for local provider install][terraformrc]:

```
provider_installation {

  dev_overrides {
      "registry.terraform.io/warpstreamlabs/warpstream" = "<PATH>"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```
where `<PATH>` should be the output of `go env GOBIN`.


[terraformrc]: https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#prepare-terraform-for-local-provider-install

Verify local installation with
```shell
go install .
cd examples/provider-install-verification
terraform plan
```

To generate or update documentation, run `go generate ./...`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

## Publish to Terraform Registry

Creating a new provider release is as simple as pushing a corresponding git tag.
The tag name must follow the Semantic Versioning standard.

Tagging a commit can be done from the git CLI or the GitHub UI. One benefit of
using the GitHub UI is that GitHub supports release notes.

To publish via GitHub, Go to this repository's [Releases Page][] to view our
release history, then click "Draft a new release".  Publishing a release will
automatically kick off a Goreleaser workflow in GitHub actions that will
publish to the Hashicorp registry. Once that workflow has completed, visit [our
listing][] in the registry to confirm that the new version is availalbe.

To publish via git

```shell
git tag v<MAJOR>.<MINOR>.<PATCH>
git push origin v<MAJOR>.<MINOR>.<PATCH>
```

See [Publishing Providers][] for details.

[Releases Page]: https://github.com/warpstreamlabs/terraform-provider-warpstream/releases/
[our listing]: https://registry.terraform.io/providers/warpstreamlabs/warpstream/latest
[Publishing Providers]: https://developer.hashicorp.com/terraform/registry/providers/publishing

