package utils

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TLS                bool   `json:"tls" yaml:"tls"`
	Key                string `json:"key" yaml:"key"`
	Cert               string `json:"cert" yaml:"cert"`
	Port               uint16 `json:"port" yaml:"port"`
	HTTP               bool   `json:"http" yaml:"http"`
	PluginDir          string `json:"plugins" yaml:"plugins"`
	ServerID           string `json:"serverId" yaml:"serverId"`
	JoinAddr           string `json:"joinAddr" yaml:"joinAddr"`
	BindAddr           string `json:"bindAddr" yaml:"bindAddr"`
	RaftBindPort       uint16 `json:"raftPort" yaml:"raftPort"`
	MemberListBindPort uint16 `json:"mlPort" yaml:"mlPort"`
	InMemory           bool   `json:"inMemory" yaml:"inMemory"`
	DataDir            string `json:"dataDir" yaml:"dataDir"`
	BootstrapCluster   bool   `json:"BootstrapCluster" yaml:"bootstrapCluster"`
	AclConfig          string `json:"AclConfig" yaml:"AclConfig"`
	RequirePass        bool   `json:"requirePass" yaml:"requirePass"`
	Password           string `json:"password" yaml:"password"`
}

func GetConfig() (Config, error) {
	tls := flag.Bool("tls", false, "Start the server in TLS mode. Default is false")
	key := flag.String("key", "", "The private key file path.")
	cert := flag.String("cert", "", "The signed certificate file path.")
	port := flag.Int("port", 7480, "Port to use. Default is 7480")
	http := flag.Bool("http", false, "Use HTTP protocol instead of raw TCP. Default is false")
	pluginDir := flag.String("pluginDir", "", "Directory where plugins are located.")
	serverId := flag.String("serverId", "1", "Server ID in raft cluster. Leave empty for client.")
	joinAddr := flag.String("joinAddr", "", "Address of cluster member in a cluster to you want to join.")
	bindAddr := flag.String("bindAddr", "", "Address to bind the server to.")
	raftBindPort := flag.Int("raftPort", 7481, "Port to use for intra-cluster communication. Leave on the client.")
	mlBindPort := flag.Int("mlPort", 7946, "Port to use for memberlist communication.")
	inMemory := flag.Bool("inMemory", false, "Whether to use memory or persisten storage for raft logs and snapshots.")
	dataDir := flag.String("dataDir", "/var/lib/memstore", "Directory to store raft snapshots and logs.")
	bootstrapCluster := flag.Bool("bootstrapCluster", false, "Whether this instance should bootstrap a new cluster.")
	aclConfig := flag.String("aclConfig", "", "ACL config file path.")
	requirePass := flag.Bool(
		"requirePass",
		false,
		"Whether the server should require a password before allowing commands. Default is false.",
	)
	password := flag.String(
		"password",
		"",
		`The password for the default user. ACL config file will overwrite this value. 
It is a plain text value by default but you can provide a SHA256 hash by adding a '#' before the hash.`,
	)

	config := flag.String(
		"config",
		"",
		`File path to a JSON or YAML config file.The values in this config file will override the flag values.`,
	)

	flag.Parse()

	conf := Config{
		TLS:                *tls,
		Key:                *key,
		Cert:               *cert,
		HTTP:               *http,
		PluginDir:          *pluginDir,
		Port:               uint16(*port),
		ServerID:           *serverId,
		JoinAddr:           *joinAddr,
		BindAddr:           *bindAddr,
		RaftBindPort:       uint16(*raftBindPort),
		MemberListBindPort: uint16(*mlBindPort),
		InMemory:           *inMemory,
		DataDir:            *dataDir,
		BootstrapCluster:   *bootstrapCluster,
		AclConfig:          *aclConfig,
		RequirePass:        *requirePass,
		Password:           *password,
	}

	if len(*config) > 0 {
		// Override configurations from file
		if f, err := os.Open(*config); err != nil {
			panic(err)
		} else {
			defer f.Close()

			ext := path.Ext(f.Name())

			if ext == ".json" {
				err := json.NewDecoder(f).Decode(&conf)
				if err != nil {
					return Config{}, nil
				}
			}

			if ext == ".yaml" || ext == ".yml" {
				err := yaml.NewDecoder(f).Decode(&conf)
				if err != nil {
					return Config{}, err
				}
			}
		}

	}

	// If requirePass is etc to true, then password must be provided as well
	var err error = nil

	if conf.RequirePass && conf.Password == "" {
		err = errors.New("password cannot be empty if requirePass is etc to true")
	}

	return conf, err
}
