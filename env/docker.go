package env

import (
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
