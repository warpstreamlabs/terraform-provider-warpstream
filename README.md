# Terraform Provider for WarpStream

_This repository is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework)._

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

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
