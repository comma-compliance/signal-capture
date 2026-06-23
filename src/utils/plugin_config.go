package utils

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type PluginConfig struct {
	Endpoint   string `yaml:"endpoint"`
	Method     string `yaml:"method"`
	ScriptPath string
}

func NewPluginConfigs() *PluginConfigs {
	return &PluginConfigs{}
}

type PluginConfigs struct {
	Configs []PluginConfig
}

func (c *PluginConfigs) Load(baseDirectory string) error {

	err := filepath.Walk(baseDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only load regular .def files from the plugin directory; skip symlinks so
		// a link inside the plugin dir cannot redirect a read outside of it.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		if filepath.Ext(path) != ".def" {
			return nil
		}

		if _, err := os.Stat(path); err == nil {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			var pluginConfig PluginConfig
			err = yaml.Unmarshal(data, &pluginConfig)
			if err != nil {
				return err
			}
			pluginConfig.ScriptPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".lua"
			c.Configs = append(c.Configs, pluginConfig)
		}
		return nil
	})

	return err
}
