package server

import (
	"bufio"
	"context"
	"daemon/config"
	"daemon/env"
	"daemon/events"
	"daemon/templates"
	"daemon/utils"
	"encoding/json"
	"errors"
	"github.com/apex/log"
	"github.com/docker/docker/api/types"
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

	Stdin types.HijackedResponse `json:"-"`
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
	Installing
	Unknown
)

var (
	stateMap = map[State]string{
		Running:    "running",
		Stopped:    "stopped",
		Starting:   "starting",
		Stopping:   "stopping",
		Installing: "installing",
		Unknown:    "unknown",
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
		State:       Stopped,
	}

<<<<<<< Updated upstream
	ev := events.New(events.ServerCreated, s.Uuid)
=======
	s.State = Unknown
	Servers = append(Servers, s)

	ev := events.New(events.ServerCreated, s)
>>>>>>> Stashed changes
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

func (s *Server) Save() error {
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

func (s *Server) tempInstallDir() string {
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

func (s *Server) Logs(follow bool) (io.ReadCloser, error) {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	reader, err := cli.ContainerLogs(ctx, s.DockerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Since:      time.Now().Format(time.RFC3339),
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

<<<<<<< Updated upstream
=======
func (p PowerAction) String() string {
	switch p {
	case PowerStart:
		return "start"
	case PowerStop:
		return "stop"
	case PowerRestart:
		return "restart"
	case PowerKill:
		return "kill"
	default:
		return "unknown"
	}
}

>>>>>>> Stashed changes
func (s *Server) Power(action PowerAction) error {
	cli, _ := env.GetDocker()
	ctx := context.Background()
	t, err := templates.GetTemplate(s.Template)
	if err != nil {
		return err
	}

	var conf struct {
		Started *string `json:"started"`
	}
	if err := json.Unmarshal([]byte(t.Docker.StartConfig), &conf); err != nil {
		return err
	}

	log.Debugf("received power action: %s for server %s", action.String(), s.Uuid)

	events.New(events.ServerLog, map[string]interface{}{
		"message": "Received power action '" + action.String() + "' for server.",
		"daemon":  true,
	}).Publish()

	switch action {
	case PowerStart:
		s.State = Starting
		events.New(events.PowerEvent, map[string]interface{}{
			"action": action.String(),
			"status": Starting.String(),
		}).Publish()
		if err := cli.ContainerStart(ctx, s.DockerId, container.StartOptions{}); err != nil {
			return err
		}

		attach, err := cli.ContainerAttach(ctx, s.DockerId, container.AttachOptions{
			Stdin:  true,
			Stdout: false,
			Stderr: false,
			Stream: true,
		})
		if err != nil {
			log.WithError(err).Fatal("failed to attach to container")
			return err
		}
		s.Stdin = attach

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
					events.New(events.PowerEvent, map[string]interface{}{
						"action": action.String(),
						"status": Running.String(),
					}).Publish()
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
		events.New(events.PowerEvent, map[string]interface{}{
			"action": action.String(),
			"status": Stopping.String(),
		}).Publish()
		go func() {
			t, err := templates.GetTemplate(s.Template)
			if err != nil {
				log.WithError(err).Fatal("failed to get template")
				return
			}

			if t.Docker.StopCommand != "" {
				cmd := t.Docker.StopCommand
				stdin := s.Stdin
				defer stdin.Close()

				if _, err := stdin.Conn.Write([]byte(cmd + "\n")); err != nil {
					log.WithError(err).Fatal("failed to write stop command to container")
					return
				}

				events.New(events.ServerLog, map[string]interface{}{
					"message": cmd,
					"daemon":  false,
				}).Publish()
			} else {
				log.Info("no stop command defined in template, skipping")
			}

			wChan, eChan := cli.ContainerWait(ctx, s.DockerId, container.WaitConditionNotRunning)
			select {
			case err := <-eChan:
				if err != nil {
					log.WithError(err).Fatal("failed to wait for container")
				}
			case <-wChan:
				s.State = Stopped
				events.New(events.PowerEvent, map[string]interface{}{
					"action": action.String(),
					"status": Stopped.String(),
				}).Publish()
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
			events.New(events.PowerEvent, map[string]interface{}{
				"action": action.String(),
				"status": Starting.String(),
			}).Publish()
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
					events.New(events.PowerEvent, map[string]interface{}{
						"action": action.String(),
						"status": Running.String(),
					})
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
	case PowerKill:
		if s.State != Stopping {
			return errors.New("server is not stopping, cannot kill")
		}

		if err := cli.ContainerKill(ctx, s.DockerId, "KILL"); err != nil {
			log.WithError(err).Error("failed to kill container")
			return err
		}

		s.State = Stopped
		events.New(events.PowerEvent, map[string]interface{}{
			"action": action.String(),
			"status": Stopped.String(),
		}).Publish()
		if err := s.Save(); err != nil {
			log.WithError(err).Fatal("failed to save server")
		}

	}

	return nil
}

func (s *Server) GetFiles(path ...string) ([]os.DirEntry, error) {
	c := *config.Get()

	volumesPath := utils.Normalize(c.VolumesPath + "/" + s.Uuid)
	if len(path) > 0 {
		volumesPath += "/" + strings.Join(path, "/")
	}

	if _, err := os.Stat(volumesPath); os.IsNotExist(err) {
		return nil, err
	}

	return os.ReadDir(volumesPath)
}

func (s *Server) GetStats() (map[string]interface{}, error) {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	info, err := cli.ContainerStats(ctx, s.DockerId, false)
	if err != nil {
		return nil, err
	}
	defer info.Body.Close()

	stats := make(map[string]interface{})
	d := json.NewDecoder(info.Body)
	if err := d.Decode(&stats); err != nil {
		return nil, err
	}

	disk, err := cli.DiskUsage(ctx, types.DiskUsageOptions{
		Types: []types.DiskUsageObject{types.ContainerObject},
	})
	if err != nil {
		return nil, err
	}

	if disk.Containers == nil || len(disk.Containers) == 0 {
		return nil, errors.New("no disk usage information available")
	}

	for _, c := range disk.Containers {
		if c.ID == s.DockerId {
			stats["disk_usage"] = c.SizeRootFs
			break
		}
	}

	inspect, err := cli.ContainerInspect(ctx, s.DockerId)
	if err != nil {
		return nil, err
	}

	if inspect.State == nil {
		return nil, errors.New("container state is nil")
	}

	simplifiedStats := map[string]interface{}{
		"cpu_usage":    stats["cpu_stats"].(map[string]interface{})["cpu_usage"].(map[string]interface{})["total_usage"],
		"cpu_max":      s.Resources.Cpu,
		"memory_usage": stats["memory_stats"].(map[string]interface{})["usage"],
		"memory_max":   s.Resources.Memory,
		"disk_usage":   stats["disk_usage"],
		"disk_max":     s.Resources.Disk,
		"status":       s.State.String(),
	}

	return simplifiedStats, nil
}

func (s *Server) GetLogsSinceStart() ([]string, error) {
	cli, _ := env.GetDocker()
	ctx := context.Background()

	inspect, err := cli.ContainerInspect(ctx, s.DockerId)
	if err != nil {
		return nil, err
	}

	if !inspect.State.Running {
		return nil, errors.New("container is not running")
	}

	start := inspect.State.StartedAt
	if start == "" {
		start = time.Now().Format(time.RFC3339)
	}

	reader, err := cli.ContainerLogs(ctx, s.DockerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Since:      start,
	})
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	var logs []string
	for scanner.Scan() {
		line := scanner.Text()
		logs = append(logs, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
