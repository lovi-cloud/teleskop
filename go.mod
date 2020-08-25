module github.com/whywaita/teleskop

go 1.14

require (
	github.com/coreos/go-iptables v0.4.5
	github.com/digitalocean/go-libvirt v0.0.0-20200320195706-d56fdc7b97e1
	github.com/golang/protobuf v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.1-0.20200613074404-b28fb2bb3547
	github.com/vishvananda/netlink v1.1.0
	github.com/whywaita/go-os-brick v0.0.8
	github.com/whywaita/satelit v0.0.0-20200811075629-01e2712998d9
	go.uber.org/zap v1.15.0
	go.universe.tf/netboot v0.0.0-20200604010521-c56445963ec8
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/sys v0.0.0-20200810151505-1b9f1253b3ed // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20200808173500-a06252235341 // indirect
	google.golang.org/grpc v1.31.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.3.0
)

replace (
	github.com/whywaita/go-os-brick v0.0.7 => ../go-os-brick
)