package server

import (
	"context"
	"daemon/config"
	"daemon/env"
	"daemon/events"
	"daemon/templates"
	"daemon/utils"
	"encoding/json"
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
	State string `json:"state"`

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

var stateMap = map[State]string{
	Running:  "running",
	Stopped:  "stopped",
	Starting: "starting",
	Stopping: "stopping",
	Unknown:  "unknown",
}

func (s State) String() string {
	return stateMap[s]
}

func GetServers() ([]Server, error) {
	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/servers")

	b, err := os.ReadDir(data)
	if err != nil {
		return []Server{}, err
	}

	servers := []Server{}
	for _, f := range b {
		b, err := os.ReadFile(data + "/" + f.Name())
		if err != nil {
			return []Server{}, err
		}

		var s Server
		if err := json.Unmarshal(b, &s); err != nil {
			return []Server{}, err
		}

		servers = append(servers, s)
	}

	return servers, nil
}

func GetServer(id string) (*Server, error) {
	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/servers")
	if len(id) <= 8 {
		files, err := os.ReadDir(data)
		if err != nil {
			return &Server{}, err
		}

		for _, f := range files {
			if strings.HasPrefix(f.Name(), id) {
				id = strings.Split(f.Name(), ".")[0]
				break
			}
		}

		if len(id) <= 8 {
			return &Server{}, os.ErrNotExist
		}
	}

	b, err := os.ReadFile(data + "/" + id + ".json")
	if err != nil {
		return &Server{}, err
	}

	var s *Server
	if err := json.Unmarshal(b, s); err != nil {
		return &Server{}, err
	}

	return s, nil
}

func GetServerDetails(s *Server) *Details {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	details := &Details{
		State: "unknown",
	}

	if s.DockerId == "" {
		return details
	}

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
