package templates

import (
	"daemon/config"
	"daemon/utils"
	"encoding/json"
	"errors"
	"os"
)

var Templates []Template

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

	Type  string   `json:"type"`
	Rules []string `json:"rules"`
}

func GetTemplates() ([]Template, error) {
	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/templates")

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
	Templates = append(Templates, t)

	c := *config.Get()
	data := utils.Normalize(c.DataPath + "/templates")

	b, err := json.Marshal(t)
	if err != nil {
		return err
	}

	if err = os.WriteFile(data+"/"+t.Uuid+".json", b, 0644); err != nil {
		return err
	}

	return nil
}

func GetTemplate(id int) (Template, error) {
	for _, t := range Templates {
		if t.Id == id {
			return t, nil
		}
	}

	return Template{}, errors.New("template not found")
}
