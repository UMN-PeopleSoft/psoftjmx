// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	//	"fmt"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/gocarina/gocsv"
	"io"
	"os"
	"strconv"
	//"strings"
	//log "github.com/inconshreveable/log15"
)

type PsoftJmxClient struct {
	Config     *JMXConfig
	Attributes *JMXAttributes
	DomainList []*PsoftDomain
	ng         *NailGunServer
}

// Uniquely defines a single PeopleSoft instance/domain
type PsoftDomain struct {
	DomainName  string
	DomainType  string
	App         string
	Env         string
	Purpose     string
	ServerName  string
	HostName    string
	ToolsVer    string
	WeblogicVer string
	JMXPort     string
	JMXUser     string
	JMXPassword string
}

// var (
// 	srvlog = log.New("module", "client")

// )

// func init() {
// 	srvlog.SetHandler(log.MultiHandler(
// 		log.StreamHandler(os.Stderr, log.LogfmtFormat()),
// 		log.LvlFilterHandler(
// 			 log.LvlError,
// 			 log.Must.FileHandler("psoftjmx.log", log.JsonFormat()))))

// }

// called on new struct
func (cli *PsoftJmxClient) CacheJMXAttributes() error {
	err := cli.Attributes.GetAttributes(cli.Config)
	if err != nil {
		return err
	}
	return nil
}

func (cli *PsoftJmxClient) InitNailGunServer() error {
	ng := &NailGunServer{
		JavaPath:         cli.Config.JavaPath,
		TransportAddress: cli.Config.NailgunServerConn,
		LogLevel:         cli.Config.LogLevel,
	}
	srvlog.Debug("Starting Nailgun Server with these parameters: " + fmt.Sprintf("%#v", ng))
	err := ng.StartNailgun()
	if err != nil {
		return err
	} else {
		cli.ng = ng
	}
	return nil
}

func (cli *PsoftJmxClient) LoadTargets(targetType string) error {
	_, err := os.Stat(cli.Config.PathInventoryFile)
	if err != nil {
		return err
	}
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = ' '
		return r // Allows use pipe as delimiter
	})
	f, err2 := os.Open(cli.Config.PathInventoryFile)
	if err2 != nil {
		return errors.New("Failed to open Inventory file")
	}
	defer f.Close()
	srvlog.Debug("Reading file ", cli.Config.PathInventoryFile)

	domainList := []*PsoftDomain{}
	filterList := []*PsoftDomain{}
	gocsv.UnmarshalWithoutHeaders(f, &domainList)
	for _, eachDomain := range domainList {
		if eachDomain.DomainType == targetType || targetType == "" {
			filterList = append(filterList, eachDomain)
		}
	}
	srvlog.Debug("Loaded these targets : " + fmt.Sprintf("%#v", filterList))
	cli.DomainList = filterList

	return nil
}

func (cli *PsoftJmxClient) GetMetrics(targetType string) ([]map[string]string, error) {

	var requests []JMXQueryRequest

	err := cli.LoadTargets(targetType)
	if err != nil {
		return []map[string]string{}, err
	}
	srvlog.Debug("GetMetrics: Loaded these Targets : " + fmt.Sprintf("%#v", &cli.DomainList))
	// TO-DO: Apply Blackout and exclusion

	// setup a new pool of workers based on target domain list
	jmxpool := NewPoolManager(len(cli.DomainList), cli.Config.ConcurrentWorkers)
	srvlog.Debug("GetMetrics: Built pool for " + strconv.Itoa(cli.Config.ConcurrentWorkers))
	// build data for the jobs
	for i := 0; i < len(cli.DomainList); i++ {
		request := JMXQueryRequest{id: i}
		request.QueryList, err = cli.Attributes.BuildQueryStrings(targetType)
		request.MetricsCfg = cli.Attributes.GetMetricConfig(targetType)
		request.NGAddress = cli.Config.NailgunServerConn
		if err != nil {
			return []map[string]string{}, err
		}

		request.Target = *cli.DomainList[i]
		srvlog.Debug("GetMetrics: Added target: " + fmt.Sprintf("%#v", cli.DomainList[i]))
		srvlog.Debug("GetMetrics: Added Config: ") // + fmt.Sprintf("%#v", request.MetricsCfg))
		requests = append(requests, request)
	}

	// send the jobs to process and NailGun connection to the pool
	jmxresponse := jmxpool.RunJobs(requests)
	responseMetrics := make([]map[string]string, 0)
	for _, eachMetric := range jmxresponse {
		responseMetrics = append(responseMetrics, eachMetric.MetricResults)
	}
	//srvlog.Debug("GetMetrics: Final map : " + fmt.Sprintf("%#v", responseMetrics))
	return responseMetrics, nil

}

func (cli *PsoftJmxClient) Close() error {

	cli.ng.StopNailGun()
	return nil
}
