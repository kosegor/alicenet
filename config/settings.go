package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/alicenet/alicenet/logging"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type bootnodeConfig struct {
	Name             string
	ListeningAddress string
	CacheSize        int
}

type chainConfig struct {
	ID                    int
	StateDbPath           string
	StateDbInMemory       bool
	TransactionDbPath     string
	TransactionDbInMemory bool
	MonitorDbPath         string
	MonitorDbInMemory     bool
}

type ethereumConfig struct {
	DefaultAccount           string
	Endpoint                 string
	EndpointMinimumPeers     uint64
	Keystore                 string
	PassCodes                string
	FactoryAddress           string
	StartingBlock            uint64
	TxMaxGasFeeAllowedInGwei uint64
	TxMetricsDisplay         bool
	ProcessingBlockBatchSize uint64
}

type transportConfig struct {
	Size                       int
	Timeout                    time.Duration
	OriginLimit                int
	PeerLimitMin               int
	PeerLimitMax               int
	FirewallMode               bool
	FirewallHost               string
	Whitelist                  string
	PrivateKey                 string
	BootNodeAddresses          string
	P2PListeningAddress        string
	LocalStateListeningAddress string
	UPnP                       bool
}

type deployConfig struct {
	Migrations     bool
	TestMigrations bool
}

type utilsConfig struct {
	Status bool
}

type validatorConfig struct {
	Repl            bool
	RewardAccount   string
	RewardCurveSpec int
	SymmetricKey    string
}

type loggingConfig struct {
	AliceNet   string
	Consensus  string
	Transport  string
	App        string
	Db         string
	Gossipbus  string
	Badger     string
	PeerMan    string
	LocalRPC   string
	Dman       string
	Peer       string
	Yamux      string
	Ethereum   string
	Main       string
	Deploy     string
	Utils      string
	Monitor    string
	Dkg        string
	Services   string
	Settings   string
	Validator  string
	MuxHandler string
	Bootnode   string
	P2pmux     string
	Status     string
	Test       string
}

type firewalldConfig struct {
	Enabled    bool
	SocketFile string
}

type ethKeyConfig struct {
	PasswordFile string
	Json         bool
	PrivateKey   string
	LightKDF     bool
}

type configuration struct {
	ConfigurationFileName string
	LoggingLevels         string // backwards compatibility
	Logging               loggingConfig
	Deploy                deployConfig
	Ethereum              ethereumConfig
	Transport             transportConfig
	Utils                 utilsConfig
	Validator             validatorConfig
	Firewalld             firewalldConfig
	Chain                 chainConfig
	BootNode              bootnodeConfig
	EthKey                ethKeyConfig
	Version               string
}

// Configuration contains all active settings.
var Configuration configuration

type s struct {
	v interface{}
}

var flagMap map[s]*pflag.Flag

// SetBinding registers a particular Flag as tied to a particular pointer.
func SetBinding(ptr interface{}, f *pflag.Flag) {
	logger := logging.GetLogger("settings")
	logger.SetLevel(logrus.WarnLevel)
	if flagMap == nil {
		flagMap = make(map[s]*pflag.Flag)
	}
	logger.Debugf("registering %q of type %q to %p", f.Name, f.Value.Type(), ptr)
	flagMap[s{ptr}] = f
}

// SetValue takes a ptr and updates the value of the flag that's pointing to it.
func SetValue(ptr, value interface{}) {
	logger := logging.GetLogger("settings")
	f, ok := flagMap[s{ptr}]
	if !ok {
		logger.Warnf("Could not find binding for %q", ptr)
	} else {
		logger.Debugf("Setting value of %q (%q) to %v", ptr, f.Name, value)

		viper.Set(f.Name, value) // Apparently the bindings don't work both directions

		val := fmt.Sprint(value)
		err := f.Value.Set(val) // This is for cobra but not sure if it matters, but don't want to risk it
		if err != nil {
			logger.Warnf("Failed to set value of flag %v to %v", f.Name, val)
		}

		logger.Infof("retrieved value is %v", f.Value.String())
	}
}

func (t transportConfig) BootNodes() []string {
	bootNodeAddresses := strings.Split(t.BootNodeAddresses, ",")
	for idx := range bootNodeAddresses {
		bootNodeAddresses[idx] = strings.TrimSpace(bootNodeAddresses[idx])
	}
	return bootNodeAddresses
}
