---
page_title: "WarpStream Provider"
description: |-
  Terraform Provider for WarpStream
---

# WarpStream Provider

Use the WarpStream provider to interact with WarpStream resources.

## API Reference

https://docs.warpstream.com/warpstream/reference/api-reference

## Configuration

You must configure the provider with a valid API key before you can use it,
which can be obtained at https://console.warpstream.com/api_keys.

## Example Usage

```terraform
terraform {
  required_providers {
    warpstream = {
      source = "warpstreamlabs/warpstream"
    }
  }
}

# Configuration-based authentication
provider "warpstream" {
  token = "YOUR_API_KEY"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `base_url` (String) Base URL for WarpStream API. May also be provided via WARPSTREAM_API_URL environment variable.
- `token` (String, Sensitive) Token for WarpStream API. May also be provided via WARPSTREAM_API_KEY environment variable.
