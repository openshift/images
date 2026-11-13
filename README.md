# OpenShift Origin Images

These images are part of the distribution.

* `base` - Foundation image shared across all other images
* `static-base` - UBI9 scratch foundation for images that ship static binaries (Containerfile.no-ca omits global CA trust; see static-base/README.md)
* `egress/dns-proxy` - A proxy for outbound DNS
* `egress/http-proxy` - A proxy for outbound HTTP requests
* `egress/router` - A router for presenting a consistent IP to external agents
* `ipfailover/keepalived` - A virtual IP management daemon image

Only images with minimal external dependencies may be added to this repository.
