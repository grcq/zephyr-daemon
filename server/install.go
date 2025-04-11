package server

import (
	"context"
	"daemon/config"
	"daemon/events"
	"daemon/templates"
	"daemon/utils"
	"github.com/apex/log"
	"github.com/docker/docker/api/types/container"
	image2 "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"io"
	"os"
	"strings"
	"time"
)

type InstallProcess struct {
	Server *Server
	client *client.Client
}

func (i *InstallProcess) installServer(reinstall bool) error {
	c := *config.Get()
	cli := i.client

	s := i.Server
	ctx := context.Background()
	if reinstall {
		// todo: remove container
		id := s.DockerId
		if err := cli.ContainerRemove(ctx, id, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: false,
			RemoveLinks:   false,
		}); err != nil {
			return err
		}

		s.Container.Installed = false
		s.DockerId = ""
		s.UpdatedAt = time.Now().Unix()
		if err := s.Save(); err != nil {
			log.WithError(err).Warnf("failed to save server %s", s.Uuid)
		}
	}

	var reader io.ReadCloser
	var err error
	if reader, err = cli.ImagePull(ctx, s.Container.Image, image2.PullOptions{}); err != nil {
		return err
	}

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return err
	}

	volumeDir := utils.Normalize(c.VolumesPath + "/" + s.Uuid)
	if _, err := os.Stat(volumeDir); os.IsNotExist(err) {
		if err := os.MkdirAll(volumeDir, 0755); err != nil {
			return err
		}
	}

	t, err := templates.GetTemplate(s.Template)
	if err != nil {
		return err
	}

	ev := events.New(events.ServerInstallStarted)
	ev.Publish()
	defer func() {
		ev := events.New(events.ServerInstallFinished, s.Uuid)
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
	defer func() {
		if err := os.RemoveAll(installDir); err != nil {
			log.WithError(err).Error("failed to remove temp install dir")
		}
	}()

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
