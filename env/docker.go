package env

import (
	"context"
	"github.com/pkg/errors"
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

func StartDocker() error {
	return nil
}
