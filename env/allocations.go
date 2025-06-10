package env

import (
	"daemon/config"
	"github.com/docker/go-connections/nat"
	"net"
	"strconv"
)

type Allocations struct {
	ForceOutgoingIp bool `json:"force_outgoing_ip"`
	DefaultMapping  struct {
		Ip   string `json:"ip"`
		Port int    `json:"port"`
	} `json:"default"`

	Mappings map[string][]int `json:"mappings"`
}

func (a *Allocations) Bindings() nat.PortMap {
	out := nat.PortMap{}

	for ip, ports := range a.Mappings {
		for _, port := range ports {
			if port < 1 || port > 65535 {
				continue
			}

			binding := nat.PortBinding{
				HostIP:   ip,
				HostPort: strconv.Itoa(port),
			}

			tcpPort := nat.Port(strconv.Itoa(port) + "/tcp")
			udpPort := nat.Port(strconv.Itoa(port) + "/udp")

			out[tcpPort] = append(out[tcpPort], binding)
			out[udpPort] = append(out[udpPort], binding)
		}
	}

	return out
}

func (a *Allocations) DockerBindings() nat.PortMap {
	c := config.Get()
	inf := c.Docker.Network.Interface
	out := a.Bindings()

	for p, bindings := range out {
		for i, binding := range bindings {
			if binding.HostIP != "127.0.0.1" {
				continue
			}

			if c.Docker.Network.ISPN {
				out[p] = append(out[p][:i], out[p][i+1:]...)
			} else {
				out[p][i] = nat.PortBinding{
					HostIP:   inf,
					HostPort: binding.HostPort,
				}
			}
		}
	}
	return out
}

func (a *Allocations) Exposed() nat.PortSet {
	out := nat.PortSet{}

	for port := range a.DockerBindings() {
		out[port] = struct{}{}
	}

	return out
}

func ShouldForceOutgoing(ipStr string) bool {
	c := config.Get()
	ip := net.ParseIP(ipStr)
	if ip == nil || ip.IsLoopback() {
		return false
	}

	if c.Docker.Network.ISPN {
		return false
	}

	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, block := range privateBlocks {
		_, cidr, _ := net.ParseCIDR(block)
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}
