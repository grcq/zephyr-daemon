package testing

import (
	"daemon/config"
	"daemon/templates"
	"encoding/json"
	"github.com/apex/log"
	"os"
)

func RunTests() {
	createTestTemplate()
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
