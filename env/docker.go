package env

import (
	"context"
	"daemon/config"
	"github.com/apex/log"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	"strconv"
	"sync"
)
import "github.com/docker/docker/client"

var (
	_once   sync.Once
	_client *client.Client
)

func GetDocker() (*client.Client, error) {
	var err error

	_once.Do(func() {
		_client, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	})
	return _client, errors.Wrap(err, "docker: unable to create client")
}

func IsDockerRunning() bool {
	cli, err := GetDocker()
	if err != nil {
		return false
	}

	_, err = cli.Ping(context.Background())
	if err != nil {
		return false
	}
	return true
}

func ConfigureDocker(ctx context.Context) error {
	cli, err := GetDocker()
	if err != nil {
		return err
	}

	c := config.Get()
	nw := c.Docker.Network
	res, err := cli.NetworkInspect(ctx, nw.Name, network.InspectOptions{})
	if err != nil {
		if !client.IsErrNotFound(err) {
			return err
		}

		log.Info("network interface does not exist, creating it... please wait")
		if err := createDockerNetwork(ctx, cli); err != nil {
			return errors.Wrap(err, "docker: failed to create network interface")
		}
	}

	c.Docker.Network.Driver = res.Driver
	switch c.Docker.Network.Driver {
	case "host":
		c.Docker.Network.Interface = "127.0.0.1"
		c.Docker.Network.ISPN = false
	case "overlay":
		fallthrough
	case "weavemesh":
		c.Docker.Network.Interface = ""
		c.Docker.Network.ISPN = true
	default:
		c.Docker.Network.ISPN = false
	}

	if err := c.Save(); err != nil {
		return errors.Wrap(err, "docker: failed to save docker config")
	}
	return nil
}

func createDockerNetwork(ctx context.Context, cli *client.Client) error {
	c := config.Get()
	nw := c.Docker.Network
	ipv6 := nw.IPv6
	_, err := cli.NetworkCreate(ctx, nw.Name, network.CreateOptions{
		Driver:     nw.Driver,
		EnableIPv6: &ipv6,
		Internal:   nw.IsInternal,
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{{
				Subnet:  nw.Interfaces.V4.Subnet,
				Gateway: nw.Interfaces.V4.Gateway,
			}, {
				Subnet:  nw.Interfaces.V6.Subnet,
				Gateway: nw.Interfaces.V6.Gateway,
			}},
		},
		Options: map[string]string{
			"encryption": "false",
			"com.docker.network.bridge.default_bridge":       "false",
			"com.docker.network.bridge.enable_icc":           strconv.FormatBool(nw.EnableICC),
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.host_binding_ipv4":    "0.0.0.0",
			"com.docker.network.bridge.name":                 "i_zephyr",
			"com.docker.network.bridge.mtu":                  strconv.FormatInt(nw.NetworkMTU, 10),
		},
	})
	if err != nil {
		return err
	}

	if nw.Driver != "host" && nw.Driver != "overlay" && nw.Driver != "weavemesh" {
		c.Docker.Network.Interface = c.Docker.Network.Interfaces.V4.Gateway
		if err := c.Save(); err != nil {
			return err
		}
	}

	return nil
}
