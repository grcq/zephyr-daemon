package server

import (
	"context"
	"daemon/config"
	"daemon/env"
	"daemon/events"
	"daemon/templates"
	"daemon/utils"
	"encoding/json"
	"errors"
	"github.com/apex/log"
	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"io"
	"os"
	"strings"
	"time"
)

type Server struct {
	Id          string `json:"id"`
	Uuid        string `json:"uuid"`
	DockerId    string `json:"docker_id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Template  int       `json:"template"`
	Container Container `json:"container"`

	Resources   Resources    `json:"resources"`
	Allocations []Allocation `json:"allocations"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`

	State State `json:"state"`
}

type Resources struct {
	Memory int64 `json:"memory"`
	Cpu    int64 `json:"cpu"`
	Disk   int64 `json:"disk"`
}

type Container struct {
	StartupCommand string            `json:"startup_command"`
	Image          string            `json:"image"`
	Installed      bool              `json:"installed"`
	Variables      map[string]string `json:"variables"`
}

type Allocation struct {
	Ip      string `json:"ip"`
	Port    int    `json:"port"`
	Primary bool   `json:"primary"`
}

type State int

const (
	Running State = iota
	Stopped
	Starting
	Stopping
	Unknown
)

var (
	stateMap = map[State]string{
		Running:  "running",
		Stopped:  "stopped",
		Starting: "starting",
		Stopping: "stopping",
		Unknown:  "unknown",
	}
	Servers []*Server
)

func (s State) String() string {
	return stateMap[s]
}

func Load(c *config.Config) {
	Servers = []*Server{}
	data := utils.Normalize(c.DataPath + "/servers")
	log.Debugf("loading servers from %s", data)

	b, err := os.ReadDir(data)
	if err != nil {
		return
	}

	for _, f := range b {
		b, err := os.ReadFile(data + "/" + f.Name())
		if err != nil {
			continue
		}

		var s Server
		if err := json.Unmarshal(b, &s); err != nil {
			log.WithError(err).Errorf("failed to load server %s", f.Name())
			continue
		}

		log.Debugf("loaded server %s", s.Uuid)

		s.State = GetState(s.DockerId)
		if err = s.Save(); err != nil {
			log.WithError(err).Errorf("failed to save server %s", s.Uuid)
		}

		Servers = append(Servers, &s)
	}
}

func GetServer(id string) (*Server, error) {
	if len(id) <= 8 {
		for _, s := range Servers {
			if s.Id == id {
				return s, nil
			}
		}

		return nil, errors.New("server not found")
	}

	for _, s := range Servers {
		if s.Uuid == id {
			return s, nil
		}
	}

	return nil, errors.New("server not found")
}

func GetState(id string) State {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	if id == "" {
		return Unknown
	}

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return Unknown
	}

	switch info.State.Status {
	case "running":
		return Running
	case "exited":
		return Stopped
	case "created":
		return Stopped
	case "dead":
		return Stopped
	}

	return Unknown
}

func CreateServer(name string, description string, template int, image string, startCommand string,
	resources Resources, allocations []Allocation, variables map[string]string) (*Server, error) {
	c := *config.Get()

	sUuid := uuid.New().String()
	id := sUuid[:8]

	s := &Server{
		Id:          id,
		Uuid:        sUuid,
		Name:        name,
		Description: description,
		Template:    template,
		Container: Container{
			StartupCommand: startCommand,
			Image:          image,
			Installed:      false,
			Variables:      variables,
		},
		Resources:   resources,
		Allocations: allocations,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	ev := events.New(events.ServerCreated, s)
	ev.Publish()

	volumesPath := utils.Normalize(c.VolumesPath + "/" + s.Uuid)
	if _, err := os.Stat(volumesPath); os.IsNotExist(err) {
		if err := os.MkdirAll(volumesPath, 0755); err != nil {
			return nil, err
		}
	}

	path := utils.Normalize(c.DataPath + "/servers")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, err
		}
	}

	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(path+"/"+s.Uuid+".json", b, 0644); err != nil {
		return nil, err
	}

	cl, _ := env.GetDocker()
	i := &InstallProcess{
		Server: s,
		client: cl,
	}
	if err := i.installServer(false); err != nil {
		return nil, err
	}

	return s, nil
}

func (s Server) Save() error {
	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/servers")

	b, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return err
	}

	if err = os.WriteFile(data+"/"+s.Uuid+".json", b, 0644); err != nil {
		return err
	}

	return nil
}

func (s Server) tempInstallDir() string {
	c := *config.Get()
	dir := utils.Normalize(c.VolumesPath + "/install_" + s.Uuid)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.WithError(err).Fatal("failed to create temp install dir")
			return ""
		}
	}

	return dir
}

func (s Server) Logs(follow bool) (io.ReadCloser, error) {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	reader, err := cli.ContainerLogs(ctx, s.DockerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
	})
	if err != nil {
		log.WithError(err).Fatal("failed to get logs")
		return nil, err
	}

	return reader, nil
}

type PowerAction int

const (
	PowerStart PowerAction = iota
	PowerStop
	PowerRestart
	PowerKill
)

func (s *Server) Power(action PowerAction) error {
	cli, _ := env.GetDocker()
	ctx := context.Background()
	t, err := templates.GetTemplate(s.Template)
	if err != nil {
		return err
	}

	var conf *struct {
		Started *string `json:"started"`
	}
	if err := json.Unmarshal([]byte(t.Docker.StartConfig), conf); err != nil {
		return err
	}

	switch action {
	case PowerStart:
		s.State = Starting
		if err := cli.ContainerStart(ctx, s.DockerId, container.StartOptions{}); err != nil {
			return err
		}

		go func() {
			if conf.Started == nil {
				return
			}

			r, err := s.Logs(true)
			if err != nil {
				log.WithError(err).Fatal("failed to get logs")
				return
			}

			// read lines and check for started message
			defer func() {
				if err := r.Close(); err != nil {
					log.WithError(err).Fatal("failed to close reader")
				}
			}()

			for {
				buf := make([]byte, 1024)
				n, err := r.Read(buf)
				if err != nil {
					log.WithError(err).Fatal("failed to read logs")
					return
				}

				if strings.Contains(string(buf[:n]), *conf.Started) {
					s.State = Running
					err := r.Close()
					if err != nil {
						log.WithError(err).Fatal("failed to close reader")
					}

					err = s.Save()
					if err != nil {
						log.WithError(err).Fatal("failed to save server")
					}
					break
				}
			}
		}()
	case PowerStop:
		s.State = Stopping
		go func() {
			wChan, eChan := cli.ContainerWait(ctx, s.DockerId, container.WaitConditionNotRunning)
			select {
			case err := <-eChan:
				if err != nil {
					log.WithError(err).Fatal("failed to wait for container")
				}
			case <-wChan:
				s.State = Stopped
				if err := s.Save(); err != nil {
					log.WithError(err).Fatal("failed to save server")
				}
			}
		}()

		if err := cli.ContainerStop(ctx, s.DockerId, container.StopOptions{}); err != nil {
			return err
		}
	case PowerRestart:
		wChan, eChan := cli.ContainerWait(ctx, s.DockerId, container.WaitConditionNotRunning)
		select {
		case err := <-eChan:
			if err != nil {
				log.WithError(err).Fatal("failed to wait for container")
			}
		case <-wChan:
			s.State = Starting
			if err := s.Save(); err != nil {
				log.WithError(err).Fatal("failed to save server")
			}
		}

		go func() {
			if conf.Started == nil {
				return
			}

			r, err := s.Logs(true)
			if err != nil {
				log.WithError(err).Fatal("failed to get logs")
				return
			}

			// read lines and check for started message
			defer func() {
				if err := r.Close(); err != nil {
					log.WithError(err).Fatal("failed to close reader")
				}
			}()

			for {
				buf := make([]byte, 1024)
				n, err := r.Read(buf)
				if err != nil {
					log.WithError(err).Fatal("failed to read logs")
					return
				}

				if strings.Contains(string(buf[:n]), *conf.Started) {
					s.State = Running
					err := r.Close()
					if err != nil {
						log.WithError(err).Fatal("failed to close reader")
					}

					err = s.Save()
					if err != nil {
						log.WithError(err).Fatal("failed to save server")
					}
					break
				}
			}
		}()

		if err := cli.ContainerRestart(ctx, s.DockerId, container.StopOptions{}); err != nil {
			return err
		}
	}

	return nil
}
