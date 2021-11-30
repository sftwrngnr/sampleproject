package TestWebServer

import (
	"fmt"
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

type TestWSYaml struct {
	Host             string `yaml:"host,omitempty"`
	Port             int    `yaml:"port"`
	User             string `yaml:"user,omitempty"`
	Password         string `yaml:"password,omitempty"`
	DaemonProc       bool   `yaml:"daemonproc"`
	WebServerRoot    string `yaml:"webserverroot"`
	TestRunnerDir    string `yaml:"testrunnerdir"`
	TestRunnerOutput string `yaml:"testrunneroutput"`
	FTPServerDir     string `yaml:"ftpserverdir"`
	SessionDir       string `yaml:"sessiondir,omitempty"`
}

func LoadTWSYaml(FileName string, TWSYaml *TestWSYaml) bool {
	log.Printf("LoadTWSYaml::loading %s\n", FileName)
	data, err := ioutil.ReadFile(FileName)
	if err != nil {
		return false
	}
	err = yaml.Unmarshal(data, TWSYaml)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return false
	}
	if TWSYaml == nil {
		return false
	}
	return true
}

func (twsy *TestWSYaml) GetServerParams() (string, int) {
	return twsy.Host, twsy.Port
}
