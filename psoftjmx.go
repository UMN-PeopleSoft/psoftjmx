// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	log "github.com/inconshreveable/log15"
	"strings"
	//"os"
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
}

var (
	defaultParallelWorkers = 5
	defaulLogLevel         = "INFO"
	defaultNGSocket        = "local:/tmp/psmetric.socket"
	srvlog                 = log.New("module", "psoftjmx")
)

const (
	psoftjmxAPIVersion = "1.0"
)

func init() {
	srvlog.SetHandler(log.LvlFilterHandler(
		log.LvlInfo,
		log.Must.FileHandler("logs/psoftjmx.log", log.LogfmtFormat())))
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
		log.Must.FileHandler("logs/psoftjmx.log", log.LogfmtFormat())))

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
	err = jmxClient.LoadTargets("")
	if err != nil {
		return nil, err
	}
	srvlog.Debug("Test Load of Targets seccessful, ready to capture metrics")
	// client is ready to handle fetch requests
	return jmxClient, nil

}
