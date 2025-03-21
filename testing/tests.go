package testing

import (
	"daemon/config"
	"daemon/server"
	"daemon/templates"
	"encoding/json"
	"github.com/apex/log"
	"os"
)

func RunTests() {
	createTestTemplate()
	//createTestServer()
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

	folder := c.DataPath + "/templates"
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
	serversPath := c.DataPath + "/servers"
	volumesPath := c.VolumesPath

	if _, err := os.Stat(serversPath); !os.IsNotExist(err) {
		log.Info("Removing existing servers")
		if err := os.RemoveAll(serversPath); err != nil {
			log.WithError(err).Fatal("failed to remove existing servers")
			return
		}
	}

	if _, err := os.Stat(volumesPath); !os.IsNotExist(err) {
		log.Info("Removing existing volumes")
		if err := os.RemoveAll(volumesPath); err != nil {
			log.WithError(err).Fatal("failed to remove existing volumes")
			return
		}
	}

	log.Info("Creating test server")
	s, err := server.CreateServer("Test Server", "This is a test server", 2, "node:20", "node .", server.Resources{
		Cpu:    100,
		Memory: 1024 * 1024 * 1024 * 1024,
		Disk:   1024 * 1024 * 1024 * 1024,
	}, []server.Allocation{
		{
			Ip:      "0.0.0.0",
			Port:    25565,
			Primary: true,
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
