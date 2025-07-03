package testing

import (
	"daemon/config"
	"daemon/env"
	"daemon/server"
	"daemon/templates"
	"encoding/json"
	"github.com/apex/log"
	"os"
)

func RunTests() {
	createTestTemplate()
	//go createTestServer()
}

func createTestTemplate() {
	c := config.Get()

	log.Info("Creating test template")
	t := templates.NewTestTemplate()

	b, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		log.WithError(err).Fatal("failed to marshal test template")
		return
	}

	folder := c.System.DataDirectory + "/templates"
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		log.Debugf("templates path.go %s does not exist, creating", folder)
		if err := os.MkdirAll(folder, 0755); err != nil {
			log.WithError(err).Fatal("failed to create templates path.go")
		}
	}

	if err := os.WriteFile(folder+"/test_template.json", b, 0644); err != nil {
		log.WithError(err).Fatal("failed to write test template")
	}

	log.Info("Test template created")
}

func createTestServer() {
	c := *config.Get()
	serversPath := c.System.DataDirectory + "/servers"
	volumesPath := c.System.VolumesDirectory

	if _, err := os.Stat(serversPath); !os.IsNotExist(err) {
		log.Debugf("Removing existing servers")
		if err := os.RemoveAll(serversPath); err != nil {
			log.WithError(err).Fatal("failed to remove existing servers")
			return
		}
	}

	if _, err := os.Stat(volumesPath); !os.IsNotExist(err) {
		log.Debugf("Removing existing volumes")
		if err := os.RemoveAll(volumesPath); err != nil {
			log.WithError(err).Fatal("failed to remove existing volumes")
			return
		}
	}

	log.Debugf("Creating test server")
	s, err := server.CreateServer("Test Server", "This is a test server", 2, "node:20", "node .", server.Resources{
		Cpu:    100,
		Memory: 1024 * 1024 * 1024 * 1024 * 2,
		Disk:   1024 * 1024 * 1024 * 1024 * 10,
	}, &env.Allocations{
		DefaultMapping: struct {
			Ip   string `json:"ip"`
			Port int    `json:"port"`
		}{
			Ip:   "127.0.0.1",
			Port: 25565,
		},
		ForceOutgoingIp: false,
		Mappings: map[string][]int{
			"127.0.0.1": {25565},
		},
	}, map[string]string{
		"test": "test",
	})
	if err != nil {
		log.WithError(err).Fatal("failed to create test server")
		return
	}

	log.WithField("server", s).Info("Test server created")
}
