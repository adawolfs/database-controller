package config


type Database struct {
	Name	string	`yaml:"name"`
	Type	string	`yaml:"type"`
	URL		string	`yaml:"url"`
	Class	string	`yaml:"class"`
}

type DBConfig struct {
	Databases map[string]Database `yaml:"Databases"`
}
