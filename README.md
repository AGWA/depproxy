# depproxy - Go module proxy that only allows authorized modules

depproxy is a Go module proxy that protects you from supply chain attacks by only proxying modules and versions that you have explicitly authorized.

## Installing

```
go install src.agwa.name/depproxy/cmd/depproxy@latest
```

Or, you can download a binary from the [GitHub Releases Page](https://github.com/AGWA/depproxy/releases).

depproxy is a single statically-linked binary so using Docker or a similar technology is superfluous.

## Module Allowlist

Your module allowlist is a text file which specifies your authorized modules.

Each line of the file contains a module path and version, separated by whitespace. Blank lines and lines starting with `#` are ignored.

The version may be `*` to allow all versions of the given module.

The module path may be a [`path.Match` pattern](https://pkg.go.dev/path#Match) to allow all matching modules.  In this case, the version must be `*`.

To allow multiple versions of a module, just specify the module on multiple lines.

### Example Allowlist

```
# This is a comment
filippo.io/age				*
github.com/aws/*			*
github.com/boltdb/bolt			v1.3.1
github.com/miekg/dns			v1.1.51
github.com/miekg/dns			v1.1.52
golang.org/x/*				*
software.sslmate.com/src/*		*
src.agwa.name/*				*
```

## Command Line Arguments

### `-allowlist FILEPATH` (Mandatory)

Read your authorized modules and versions from the given file, documented above.

You must restart depproxy after modifying this file.

### `-listen LISTENER` (Mandatory)

Listen on the given address, provided in [go-listener syntax](https://pkg.go.dev/src.agwa.name/go-listener#readme-listener-syntax).  You can specify the `-listen` flag multiple times to listen on multiple addresses.

Examples:
* `-listen tcp:8080` to listen on TCP port 8080, all interfaces.
* `-listen tls:depproxy.example.com:tcp:443` to listen on TCP port 443, all interfaces, with an automatically-obtained TLS certificate for depproxy.example.com (requires depproxy.example.com to be publicly-accessible).
* `-listen tls:/path/to/certificate.pem:tcp:443` to listen on TCP port 443, all interfaces, using a certificate chain and private key in a PEM file.

### `-upstream URL` (Optional)

Specifies the URL of the upstream Go proxy.  Default: `https://proxy.golang.org`

## Usage

Set the `GOPROXY` environment variable to the URL of your depproxy instance, followed by `/proxy`.  For example:

* `GOPROXY=https://depproxy.example.com/proxy`
* `GOPROXY=http://192.0.2.1:8080/proxy`

After you set `GOPROXY`, the go command will only be able to download modules and versions that are allowed by your allowlist.

## Web Interface

Visit your depproxy instance in a web browser to see if any of your authorized modules have newer versions.  If a newer version is available, the module will be highlighted in red and the following functions will be available to help you vet the new version:

* **Raw** - view a raw diff between the authorized version and the latest version
* **HTML** - view an HTML diff between the authorized version and the latest version
* **VCS** - view a changelog between the authorized version and the latest version in the module's version control system (only available if the module is hosted on GitHub; not available with older module versions)

After vetting the new version, edit your allowlist to specify the new version and restart depproxy.

### Screenshot

![Screenshot of web interface showing the status of your authorized modules](/doc/webapp_screenshot.png)
