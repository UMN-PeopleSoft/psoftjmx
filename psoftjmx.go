// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	log "github.com/inconshreveable/log15"
	"strings"
	"os"
)

// core configuration settings to pull metrics
type JMXConfig struct {
	PathInventoryFile   string
	PathBlackoutFile    string
	PathExclusionFile   string
	AttribWebMetrics    string
	AttribAppMetrics    string
	AttribPrcMetrics    string
	LogLevel            string
	ConcurrentWorkers   int
	NailgunServerConn   string
	JavaPath            string
	DomainInventoryFile string
	ConcatenateDomainWithHost bool
	UseLastXCharactersOfHost int
	LocalInventoryOnly  bool
	
}

var (
	defaultConcatwithHost  = false
   defaultLastNumChars    = 0
   defaultLocalInventory  = false
	defaultParallelWorkers = 5
	defaulLogLevel         = "INFO"
	logFile                = "logs/psoftjmx.log"
	srvlog                 = log.New("module", "psoftjmx")
	defaultNGSocket        = ""
)

const (
	psoftjmxAPIVersion = "1.1"
)

func init() {
	wd, _ := os.Getwd()
	_ = os.MkdirAll(wd + "/logs", 0700)
	_ = os.MkdirAll(wd + "/run", 0700)
	defaultNGSocket = "local:" + wd + "run/psmetric.socket"
	
	srvlog.SetHandler(log.LvlFilterHandler(
		log.LvlInfo,
		log.Must.FileHandler(logFile, log.LogfmtFormat())))
}

func NewClient(config *JMXConfig) (*PsoftJmxClient, error) {
	jmxClient := &PsoftJmxClient{}
	if config.ConcurrentWorkers == 0 {
		config.ConcurrentWorkers = defaultParallelWorkers
	}
	if config.LogLevel == "" {
		config.LogLevel = defaulLogLevel
	}
	if config.NailgunServerConn == "" {
		config.NailgunServerConn = defaultNGSocket
	}
	if config.UseLastXCharactersOfHost == 0 {
		config.UseLastXCharactersOfHost = defaultLastNumChars
	}
		
	// conver standard java Log level to go log level
	logStr := strings.ToLower(config.LogLevel)
	if logStr == "all" {
		logStr = "debug"
	}
	if logStr == "warning" {
		logStr = "warn"
	}
	logcode, _ := log.LvlFromString(logStr)
	srvlog.SetHandler(log.LvlFilterHandler(
		logcode,
		log.Must.FileHandler(logFile, log.LogfmtFormat())))

	srvlog.Debug("Loading Configuration for client")
	jmxClient.Config = config
	// preload-verify JMX attribute configs
	jmxClient.Attributes = new(JMXAttributes)
	err := jmxClient.CacheJMXAttributes()
	if err != nil {
		return nil, err
	}
	srvlog.Debug("Cached Attributes/Metrics")
	// startup and verify the NailGun server is running
	err = jmxClient.InitNailGunServer()
	if err != nil {
		return nil, err
	}
	srvlog.Debug("Started NailGun Server")
	// verify valid domain inventory file, will reload before calling fetch
	err = jmxClient.LoadTargets()
	if err != nil {
		return nil, err
	}
	srvlog.Debug("Test Load of Targets seccessful, ready to capture metrics")
	// client is ready to handle fetch requests
	return jmxClient, nil

}
