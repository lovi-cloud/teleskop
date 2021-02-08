# Teleskop: Agent of [lovi-cloud](https://github.com/lovi-cloud)

![logo](./docs/image/teleskop-logo.png)

## Features

- hypervisor agent
- [cloud-init](https://cloudinit.readthedocs.io/en/latest/) server
    - implemented data source as a [NoCloud](https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html)

## Getting Started

teleskop requires root permission.

```bash
## build
$ go generate ./...
$ go build .

## route to metadata server
$ sudo ip addr add 169.254.169.254/32 dev lo

## run
$ teleskop -help
Usage of /usr/local/bin/teleskop:
  -intf string
        teleskop listen interface (default "bond0.1000")
  -satelit string
        satelit datastore api endpoint (default "127.0.0.1:9263")
   
```

### systemd unit file

```bash
$ cat teleskop.service
[Unit]
Description=Teleskop Agent

[Service]
User=root
ExecStart=/usr/local/bin/teleskop -satelit 192.0.2.100:9263 -intf eth0
Restart=always

[Install]
WantedBy=multi-user.target
```