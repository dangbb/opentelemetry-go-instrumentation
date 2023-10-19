package process

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type JobConfig struct {
	Name        string `yaml:"name"`
	BinaryPath  string `yaml:"binary_path"`
	ServiceName string `yaml:"service_name"`
}

type JobsConfig struct {
	Jobs []JobConfig `yaml:"jobs"`
}

func ParseJobConfig(path string) JobsConfig {
	config := JobsConfig{}

	filename, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	file, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Done marshal file\n")
	return config
}
