package config

import (
	"io/ioutil"
	"errors"
	"fmt"
	"log"
	"gopkg.in/yaml.v2"
)


type DatabaseConfig struct {
	Name	string	`yaml:"name"`
	Type	string	`yaml:"kind"`
	URL		string	`yaml:"url"`
	Class	string	`yaml:"class"`
}

type DBConfig struct {
	Databases	[]DatabaseConfig	`yaml:"databases"`
}

func read_config(filename string) (*DBConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	dbconfig := new(DBConfig)
	err = yaml.Unmarshal([]byte(data), dbconfig)
	if (err != nil) {
		return nil, err
	}

	for _, database := range dbconfig.Databases {
		if database.Name == "" {
			return nil, errors.New("Database server missing 'name'")
		}
		if database.URL == "" {
			return nil, errors.New(fmt.Sprintf(`Database server "%s" missing URL`,
				database.Name))
		}
		if database.Type == "" {
			return nil, errors.New("Database server missing 'type'")
		}
		if database.Class == "" {
			database.Class = "default"
			log.Printf(`note: Database server "%s" missing class; set to "default"`,
				database.Name)
		}
	}

	return dbconfig, nil
}

