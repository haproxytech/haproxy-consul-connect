## v0.2.0 (2020-05-06)

FEATURES:

- Add appname_header option - #41
- Add enable_forwardfor option - #40
- Add support for "mode tcp" - #16
- Check dataplaneapi and haproxy binaries - #31

IMPROVEMENTS:

- Update go min version to 1.13 - #43
- Update Consul APIs to 1.4.0 and Consul to 1.7.2 - #28
- Added version in build - #24
- Plug in logs from Consul client - #18
- Perform state diff to configure haproxy: avoid config desync and better backend server handling

BUGFIXES:

- Properly detect when helper processes are not in the `$PATH` (eg: dataplaneapi or haproxy) - #21

## v0.1.9 (2019-09-25)

First initial release.

Supports only HTTP Connections for now.
