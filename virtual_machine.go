package main

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	libvirt "github.com/digitalocean/go-libvirt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/whywaita/teleskop/protoc/agent"
)

const domainTmplStr = `
<domain type='kvm' xmlns:qemu='http://libvirt.org/schemas/domain/qemu/1.0'>
  <name>{{.Name}}</name>
  <memory unit='KiB'>{{.MemoryKib}}</memory>
  <currentMemory unit='KiB'>{{.MemoryKib}}</currentMemory>
  <vcpu placement='static'>{{.Vcpus}}</vcpu>
  <resource>
    <partition>/machine</partition>
  </resource>
  <sysinfo type='smbios'>
    <system>
      <entry name='manufacturer'>CyberAgent, Inc.</entry>
    </system>
  </sysinfo>
  <os>
    <type arch='x86_64' machine='pc-i440fx-bionic'>hvm</type>
    <boot dev='hd'/>
    <smbios mode='sysinfo'/>
  </os>
  <features>
    <acpi/>
    <apic/>
  </features>
  <cpu mode='host-model'/>
  <clock offset='utc'>
    <timer name='pit' tickpolicy='delay'/>
    <timer name='rtc' tickpolicy='catchup'/>
    <timer name='hpet' present='no'/>
  </clock>
  <on_poweroff>destroy</on_poweroff>
  <on_reboot>restart</on_reboot>
  <on_crash>destroy</on_crash>
  <devices>
    <emulator>/usr/bin/kvm-spice</emulator>
    <controller type='usb' index='0' model='piix3-uhci'>
      <alias name='usb'/>
    </controller>
    <controller type='pci' index='0' model='pci-root'>
      <alias name='pci.0'/>
    </controller>
    <input type='tablet' bus='usb'>
      <alias name='input0'/>
      <address type='usb' bus='0' port='1'/>
    </input>
    <input type='mouse' bus='ps2'>
      <alias name='input1'/>
    </input>
    <input type='keyboard' bus='ps2'>
      <alias name='input2'/>
    </input>
    <video>
      <model type='cirrus' vram='16384' heads='1' primary='yes'/>
      <alias name='video0'/>
    </video>
    <serial type='pty'>
      <target port='0'/>
      <alias name='serial0'/>
    </serial>
    <console type='pty' >
      <target type='serial' port='0'/>
      <alias name='serial0'/>
    </console>
  </devices>
  <qemu:commandline>
    <qemu:arg value='-smbios'/>
    <qemu:arg value='type=1,serial=ds=nocloud-net;s=http://169.254.169.254/'/>
  </qemu:commandline>
</domain>
`

const attachDiskTmplStr = `
<disk type='block'>
  <source dev='{{.SourceDevice}}'/>
  <target dev='{{.TargetDevice}}'/>
  <iotune>
  {{ if gt .ReadBytesSec 0 }}<read_bytes_sec>{{ .ReadBytesSec }}</read_bytes_sec>{{ end }}
  {{ if gt .WriteBytesSec 0 }}<write_bytes_sec>{{ .WriteBytesSec }}</write_bytes_sec>{{ end }}
  {{ if gt .ReadIopsSec 0 }}<read_iops_sec>{{ .ReadIopsSec }}</read_iops_sec>{{ end }}
  {{ if gt .WriteIopsSec 0 }}<write_iops_sec>{{ .WriteIopsSec }}</write_iops_sec>{{ end }}
  </iotune>
</disk>
`

const detachDiskTmplStr = `
<disk type='block'>
  <source dev='{{.SourceDevice}}'/>
  <target dev='{{.TargetDevice}}'/>
</disk>
`

const interfaceTmplStr = `
<interface type='bridge'>
  <mac address='{{.MacAddress}}'/>
  <source bridge='{{.Bridge}}'/>
  <model type='virtio'/>
  <target dev='{{.Name}}'/>
  <bandwidth>
    <inbound average='{{.InboundAverage}}'/>
    <outbound average='{{.OutboundAverage}}'/>
  </bandwidth>
</interface>
`

var (
	domainTmpl     *template.Template
	attachDiskTmpl *template.Template
	detachDiskTmpl *template.Template
	interfaceTmpl  *template.Template
)

func (a *agent) AddVirtualMachine(ctx context.Context, req *pb.AddVirtualMachineRequest) (*pb.AddVirtualMachineResponse, error) {
	if domainTmpl == nil {
		tmp, err := template.New("domainTmpl").Parse(domainTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse domain template: %+v", err)
		}
		domainTmpl = tmp
	}

	var buff bytes.Buffer
	if err := domainTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to exec domain template: %+v", err)
	}

	domain, err := a.libvirtClient.DomainDefineXML(buff.String())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to define domain: %+v", err)
	}

	fmt.Printf("creating domain: %s\t%x\n", domain.Name, domain.UUID)

	if req.BootDevice != "" {
		_, err = a.AttachBlockDevice(ctx, &pb.AttachBlockDeviceRequest{
			Uuid:          fmt.Sprintf("%x", domain.UUID),
			SourceDevice:  req.BootDevice,
			TargetDevice:  "vda",
			ReadBytesSec:  req.ReadBytesSec,
			WriteBytesSec: req.WriteBytesSec,
			ReadIopsSec:   req.ReadIopsSec,
			WriteIopsSec:  req.WriteIopsSec,
		})
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "failed to attach boot device: %+v", err)
		}
	}

	return &pb.AddVirtualMachineResponse{
		Uuid: fmt.Sprintf("%x", domain.UUID),
		Name: domain.Name,
	}, nil
}

func (a *agent) StartVirtualMachine(ctx context.Context, req *pb.StartVirtualMachineRequest) (*pb.StartVirtualMachineResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if err := a.libvirtClient.DomainCreate(*domain); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to start domain: %+v", err)
	}

	fmt.Printf("starting domain: %s\t%x\n", domain.Name, domain.UUID)

	return &pb.StartVirtualMachineResponse{
		Uuid: fmt.Sprintf("%x", domain.UUID),
		Name: domain.Name,
	}, nil
}

func (a *agent) AttachBlockDevice(ctx context.Context, req *pb.AttachBlockDeviceRequest) (*pb.AttachBlockDeviceResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if attachDiskTmpl == nil {
		tmp, err := template.New("attachDiskTmpl").Parse(attachDiskTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse disk template: %+v", err)
		}
		attachDiskTmpl = tmp
	}

	var buff bytes.Buffer
	if err := attachDiskTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to exec disk template: %+v", err)
	}

	state, err := a.getDomainState(ctx, *domain)
	if err != nil {
		return nil, err
	}
	var flags libvirt.DomainDeviceModifyFlags
	switch libvirt.DomainState(state) {
	case libvirt.DomainRunning:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyLive
	case libvirt.DomainShutoff:
		flags = libvirt.DomainDeviceModifyConfig
	default:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyForce
	}

	if err := a.libvirtClient.DomainAttachDeviceFlags(*domain, buff.String(), uint32(flags)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to attach block device: %+v", err)
	}

	fmt.Printf("attaching block: %s\t%x\n", domain.Name, domain.UUID)

	return &pb.AttachBlockDeviceResponse{
		Uuid: fmt.Sprintf("%x", domain.UUID),
		Name: domain.Name,
	}, nil
}

func (a *agent) AttachInterface(ctx context.Context, req *pb.AttachInterfaceRequest) (*pb.AttachInterfaceResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if interfaceTmpl == nil {
		tmp, err := template.New("interfaceTmpl").Parse(interfaceTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse interface template: %+v", err)
		}
		interfaceTmpl = tmp
	}

	var buff bytes.Buffer
	if err := interfaceTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to exec interface template: %+v", err)
	}

	state, err := a.getDomainState(ctx, *domain)
	if err != nil {
		return nil, err
	}
	var flags libvirt.DomainDeviceModifyFlags
	switch libvirt.DomainState(state) {
	case libvirt.DomainRunning:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyLive
	case libvirt.DomainShutoff:
		flags = libvirt.DomainDeviceModifyConfig
	default:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyForce
	}

	if err := a.libvirtClient.DomainAttachDeviceFlags(*domain, buff.String(), uint32(flags)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to attach interface: %+v", err)
	}

	fmt.Printf("attaching interface: %s\t%x\n", domain.Name, domain.UUID)

	return &pb.AttachInterfaceResponse{
		Uuid: fmt.Sprintf("%x", domain.UUID),
		Name: domain.Name,
	}, nil
}

func (a *agent) DeleteVirtualMachine(ctx context.Context, req *pb.DeleteVirtualMachineRequest) (*pb.DeleteVirtualMachineResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if err := a.libvirtClient.DomainUndefine(*domain); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to undefine domain: %+v", err)
	}

	return &pb.DeleteVirtualMachineResponse{}, nil
}

func (a *agent) StopVirtualMachine(ctx context.Context, req *pb.StopVirtualMachineRequest) (*pb.StopVirtualMachineResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if err := a.libvirtClient.DomainDestroy(*domain); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to destory domain: %+v", err)
	}

	return &pb.StopVirtualMachineResponse{}, nil
}

func (a *agent) DetachBlockDevice(ctx context.Context, req *pb.DetachBlockDeviceRequest) (*pb.DetachBlockDeviceResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if detachDiskTmpl == nil {
		tmp, err := template.New("detachDiskTmpl").Parse(detachDiskTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse disk template: %+v", err)
		}
		detachDiskTmpl = tmp
	}

	var buff bytes.Buffer
	if err := detachDiskTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to exec disk template: %+v", err)
	}

	state, err := a.getDomainState(ctx, *domain)
	if err != nil {
		return nil, err
	}
	var flags libvirt.DomainDeviceModifyFlags
	switch libvirt.DomainState(state) {
	case libvirt.DomainRunning:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyLive
	case libvirt.DomainShutoff:
		flags = libvirt.DomainDeviceModifyConfig
	default:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyForce
	}

	if err := a.libvirtClient.DomainDetachDeviceFlags(*domain, buff.String(), uint32(flags)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to detach block device: %+v", err)
	}

	return &pb.DetachBlockDeviceResponse{}, nil
}

func (a *agent) DetachInterface(ctx context.Context, req *pb.DetachInterfaceRequest) (*pb.DetachInterfaceResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	if interfaceTmpl == nil {
		tmp, err := template.New("interfaceTmpl").Parse(interfaceTmplStr)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to parse interface template: %+v", err)
		}
		interfaceTmpl = tmp
	}

	var buff bytes.Buffer
	if err := interfaceTmpl.Execute(&buff, req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to exec interface template: %+v", err)
	}

	state, err := a.getDomainState(ctx, *domain)
	if err != nil {
		return nil, err
	}
	var flags libvirt.DomainDeviceModifyFlags
	switch libvirt.DomainState(state) {
	case libvirt.DomainRunning:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyLive
	case libvirt.DomainShutoff:
		flags = libvirt.DomainDeviceModifyConfig
	default:
		flags = libvirt.DomainDeviceModifyConfig | libvirt.DomainDeviceModifyForce
	}

	if err := a.libvirtClient.DomainDetachDeviceFlags(*domain, buff.String(), uint32(flags)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to detach interface: %+v", err)
	}

	return &pb.DetachInterfaceResponse{}, nil
}

func (a *agent) GetVirtualMachineState(ctx context.Context, req *pb.GetVirtualMachineStateRequest) (*pb.GetVirtualMachineStateResponse, error) {
	domain, err := a.domainLookupByUUID(req.Uuid)
	if err != nil {
		return nil, err
	}

	state, _, err := a.libvirtClient.DomainGetState(*domain, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get domain state: %+v", err)
	}

	return &pb.GetVirtualMachineStateResponse{
		State: &pb.VirtualMachineState{
			Uuid:  fmt.Sprintf("%x", domain.UUID),
			Name:  domain.Name,
			State: pb.VirtualMachineState_State(state),
		},
	}, nil

}

func (a *agent) ListVirtualMachineState(ctx context.Context, req *pb.ListVirtualMachineStateRequest) (*pb.ListVirtualMachineStateResponse, error) {
	flags := libvirt.ConnectListDomainsActive | libvirt.ConnectListDomainsInactive
	domains, _, err := a.libvirtClient.ConnectListAllDomains(1, flags)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get domain list: %+v", err)
	}

	vms := make([]*pb.VirtualMachineState, len(domains))
	for i, domain := range domains {
		state, _, err := a.libvirtClient.DomainGetState(domain, 0)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get domain state: %+v", err)
		}
		vms[i] = &pb.VirtualMachineState{
			Uuid:  fmt.Sprintf("%x", domain.UUID),
			Name:  domain.Name,
			State: pb.VirtualMachineState_State(state),
		}
	}

	return &pb.ListVirtualMachineStateResponse{
		States: vms,
	}, nil
}

func (a *agent) getDomainState(ctx context.Context, domain libvirt.Domain) (int32, error) {
	state, _, err := a.libvirtClient.DomainGetState(domain, 0)
	if err != nil {
		return -1, status.Errorf(codes.Internal, "failed to get domain stat: %+v", err)
	}

	return state, nil
}
