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
	image2 "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
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

	Details Details `json:"-"`
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

type Details struct {
	State State `json:"state"`

	Usage struct {
		Cpu    float64 `json:"cpu"`
		Memory float64 `json:"memory"`
		Disk   float64 `json:"disk"`
	} `json:"usage"`
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

func init() {
	Servers = []*Server{}
	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/servers")

	b, err := os.ReadDir(data)
	if err != nil {
		return
	}

	for _, f := range b {
		b, err := os.ReadFile(data + "/" + f.Name())
		if err != nil {
			continue
		}

		var s *Server
		if err := json.Unmarshal(b, s); err != nil {
			continue
		}

		s.Details = *GetServerDetails(s.DockerId)
		Servers = append(Servers, s)
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

func GetServerDetails(id string) *Details {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	details := &Details{
		State: Unknown,
	}

	if id == "" {
		return details
	}

	info, err := cli.ContainerInspect(ctx, id)
	if err != nil {
		return details
	}

	switch info.State.Status {
	case "running":
		details.State = Running
	case "exited":
		details.State = Stopped
	case "created":
		details.State = Starting
	case "dead":
		details.State = Stopping
	}

	// todo: usage
	return details
}

type InstallProcess struct {
	Server *Server
	client *client.Client
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

func (i *InstallProcess) installServer(reinstall bool) error {
	c := *config.Get()
	cli := i.client

	s := i.Server
	ctx := context.Background()
	if reinstall {
		// todo: remove container
	}

	var reader io.ReadCloser
	var err error
	if reader, err = cli.ImagePull(ctx, s.Container.Image, image2.PullOptions{}); err != nil {
		return err
	}

	if _, err := io.Copy(os.Stdout, reader); err != nil {

	}

	volumeDir := utils.Normalize(c.VolumesPath + "/" + s.Uuid)
	log.Debugf("volume dir: %s", volumeDir)
	if _, err := os.Stat(volumeDir); os.IsNotExist(err) {
		if err := os.MkdirAll(volumeDir, 0755); err != nil {
			return err
		}
	}

	log.Debugf("template: %d", s.Template)
	t, err := templates.GetTemplate(s.Template)
	if err != nil {
		return err
	}

	ev := events.New(events.ServerInstallStarted)
	ev.Publish()
	defer func() {
		ev := events.New(events.ServerInstallFinished, s)
		ev.Publish()
	}()

	var envs []string
	for k, v := range s.Container.Variables {
		envs = append(envs, k+"="+v)
	}

	containerConfig := &container.Config{
		Hostname:     "installer",
		Image:        s.Container.Image,
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		OpenStdin:    true,
		Env:          envs,
		Cmd: []string{
			"sh",
			"/mnt/install/install.sh",
		},
	}

	installDir := s.tempInstallDir()
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Target:   "/mnt/data",
				Source:   strings.ReplaceAll(volumeDir, "\\", "/"),
				Type:     mount.TypeBind,
				ReadOnly: false,
			},
			{
				Target:   "/mnt/install",
				Source:   strings.ReplaceAll(installDir, "\\", "/"),
				Type:     mount.TypeBind,
				ReadOnly: false,
			},
		},
		Resources: container.Resources{
			Memory:    s.Resources.Memory,
			CPUShares: s.Resources.Cpu,
		},
	}
	/*defer func() {
		if err := os.RemoveAll(installDir); err != nil {
			log.WithError(err).Fatal("failed to remove temp install dir")
		}
	}()*/

	log.Debugf("installing server %s", s.Uuid)
	installScript := []byte(t.InstallScript)
	if err = os.WriteFile(installDir+"\\install.sh", installScript, 0644); err != nil {
		return err
	}

	var response container.CreateResponse
	if response, err = cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, s.Uuid+"_install"); err != nil {
		return err
	}
	i.Server.DockerId = response.ID

	if err = cli.ContainerStart(ctx, response.ID, container.StartOptions{}); err != nil {
		return err
	}

	go func(id string) {
		if err := i.Output(ctx, id); err != nil {
			log.WithError(err).Fatal("failed to output")
		}
	}(response.ID)

	sChan, eChan := cli.ContainerWait(ctx, response.ID, container.WaitConditionNotRunning)
	select {
	case err := <-eChan:
		if err != nil {
			return err
		}
	case <-sChan:
		log.Debugf("install finished for %s", s.Uuid)
		if err := cli.ContainerRemove(ctx, response.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: false,
			RemoveLinks:   false,
		}); err != nil {
			return err
		}

		containerConfig.WorkingDir = "/mnt/data"
		containerConfig.Hostname = s.Uuid
		containerConfig.Cmd = strings.Split(s.Container.StartupCommand, " ")

		if response, err = cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, s.Uuid); err != nil {
			return err
		}

		i.Server.DockerId = response.ID
		if err = cli.ContainerStart(ctx, response.ID, container.StartOptions{}); err != nil {
			return err
		}

		s.Container.Installed = true
		s.UpdatedAt = time.Now().Unix()

		if err := s.Save(); err != nil {
			return err
		}
	}

	return nil
}

func (i *InstallProcess) Output(ctx context.Context, id string) error {
	c := *config.Get()
	cli := i.client
	reader, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return err
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			log.WithError(err).Fatal("failed to close reader")
		}
	}(reader)

	installLog := utils.Normalize(c.VolumesPath + "/" + i.Server.Uuid + "/install.log")
	if _, err := os.Create(installLog); err != nil {
		return err
	}

	file, err := os.OpenFile(installLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, reader); err != nil {
		return err
	}

	return nil
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
		s.Details.State = Starting
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
					s.Details.State = Running
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
		s.Details.State = Stopping
		go func() {
			wChan, eChan := cli.ContainerWait(ctx, s.DockerId, container.WaitConditionNotRunning)
			select {
			case err := <-eChan:
				if err != nil {
					log.WithError(err).Fatal("failed to wait for container")
				}
			case <-wChan:
				s.Details.State = Stopped
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
			s.Details.State = Starting
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
					s.Details.State = Running
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
