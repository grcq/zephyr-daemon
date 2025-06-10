package templates

import (
	"daemon/config"
	"daemon/utils"
	"encoding/json"
	"errors"
	"os"
)

type Template struct {
	Id          int    `json:"id"`
	Uuid        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`

	Docker    Docker     `json:"docker"`
	Variables []Variable `json:"variables"`

	InstallScript string `json:"install_script"`
}

type Docker struct {
	Images []string `json:"images"`

	StartCommand string `json:"start_command"`
	StopCommand  string `json:"stop_command"`

	StartConfig string       `json:"start_config"`
	ConfigFiles []ConfigFile `json:"config_files"`
}

type ConfigFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Variable struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	EnvironmentName string `json:"environment_name"`
	DefaultValue    string `json:"default_value"`

	Type  string   `json:"type"`
	Rules []string `json:"rules"`
}

func GetTemplates() ([]Template, error) {
	c := *config.Get()
	data := utils.Normalize(c.System.DataDirectory + "/templates")

	b, err := os.ReadDir(data)
	if err != nil {
		return []Template{}, err
	}

	templates := []Template{}
	for _, f := range b {
		file, err := os.ReadFile(data + "/" + f.Name())
		if err != nil {
			return []Template{}, err
		}

		var t Template
		if err := json.Unmarshal(file, &t); err != nil {
			return []Template{}, err
		}

		templates = append(templates, t)
	}

	return templates, nil
}

func AddTemplate(t Template) error {
	c := *config.Get()
	data := utils.Normalize(c.System.DataDirectory + "/templates")

	b, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		return err
	}

	if err = os.WriteFile(data+"/"+t.Uuid+".json", b, 0644); err != nil {
		return err
	}

	return nil
}

func GetTemplate(id int) (Template, error) {
	templates, _ := GetTemplates()
	for _, t := range templates {
		if t.Id == id {
			return t, nil
		}
	}

	return Template{}, errors.New("template not found")
}

func NewTestTemplate() Template {
	return Template{
		Id:          1,
		Uuid:        "testing",
		Name:        "Test",
		Description: "Test description",

		Docker: Docker{
			Images: []string{"testing"},

			StartCommand: "testing",
			StopCommand:  "testing",

			StartConfig: "{\"started\": \"is now running!\"}",
			ConfigFiles: []ConfigFile{
				{
					Path: "server.properties",
					Content: `
#Minecraft server properties
#Thu Jan 01 00:00:00 CET 1970
server-ip=0.0.0.0
server-port={$PORT}
`,
				},
			},
		},
		Variables: []Variable{
			{
				Name:            "Server Jar",
				Description:     "The server JAR file to use",
				EnvironmentName: "SERVER_JAR",
				DefaultValue:    "",

				Type:  "string",
				Rules: []string{"required", "regex:.*\\.jar"},
			},
		},

		InstallScript: `
#!/bin/bash
echo "Installing server..."
echo "Downloading server jar..."
wget https://example.com/server.jar
echo "Server installed!"
`,
	}
}
