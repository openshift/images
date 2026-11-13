# Static base images

Base images for statically-linked OpenShift executables:

- `Containerfile`: UBI9 packages in a scratch image, with CA trust and timezone data.
- `Containerfile.no-ca`: no global CA trust; applications must mount their own.
- `Containerfile.okd`: public `hi/static` with OpenShift labels.

The UBI9 images retain `filesystem`, `setup`, `tzdata`, and optionally
`ca-certificates`, plus RPM signing-key records. Build dependencies are removed;
there is no shell, glibc, core utilities or package manager. CA-update scripts
are not usable at runtime. No user, home or working directory is prescribed.

RHEL identification files (`/etc/os-release`, `/usr/lib/os-release`, and
`/etc/redhat-release`) and distribution labels support OS detection by scanners.
The release package itself and `/usr/share/licenses` are not included.

The no-CA image preserves empty `/etc/pki` directories for OpenShift secrets.
Go applications can use a mounted PEM bundle through `SSL_CERT_FILE`.

Builds require access to public UBI9 repositories. OKD uses hi/static for its
supported architectures; the no-CA variant is disabled for OKD.

```sh
podman build -t static-base -f static-base/Containerfile static-base
podman build -t static-base-no-ca -f static-base/Containerfile.no-ca static-base
```
