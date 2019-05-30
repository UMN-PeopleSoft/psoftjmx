// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/gocarina/gocsv"
	"io"
	"os"
	"strconv"
)

type BlackoutType struct {
	DomainEnv string // the domain or env the blackout applies to
	EndTime   string // End of the blackout
	Descr     string // Reason, not used
}

type ExcludeDomainType struct {
	DomainName string
}

type PsoftJmxClient struct {
	Config     *JMXConfig
	Attributes *JMXAttributes
	DomainList []*PsoftDomain
	ng         *NailGunServer
	Blackouts  []*BlackoutType      // list of domains or envs in a blackout
	Excludes   []*ExcludeDomainType // exiting domains to skip for monitoring, ie only used for peak loads

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

func (cli *PsoftJmxClient) LoadTargets() error {
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
	gocsv.UnmarshalWithoutHeaders(f, &domainList)
	srvlog.Debug("Loaded these targets : " + fmt.Sprintf("%#v", domainList))
	cli.DomainList = domainList

	return nil
}

func (cli *PsoftJmxClient) LoadBlackouts() error {
	blackoutList := []*BlackoutType{}

	_, err := os.Stat(cli.Config.PathBlackoutFile)
	if err != nil {
		return err
	}
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = '|'
		return r // Allows use pipe as delimiter
	})
	f, err2 := os.Open(cli.Config.PathBlackoutFile)
	if err2 != nil {
		return errors.New("Failed to open Blackout file")
	}
	defer f.Close()
	srvlog.Debug("Reading file ", cli.Config.PathBlackoutFile)

	gocsv.UnmarshalWithoutHeaders(f, &blackoutList)
	srvlog.Debug("Loaded these blackout items : " + fmt.Sprintf("%#v", blackoutList))
	cli.Blackouts = blackoutList

	return nil
}

func (cli *PsoftJmxClient) LoadExclusions() error {
	_, err := os.Stat(cli.Config.PathExclusionFile)
	if err != nil {
		return err
	}
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		return r // Allows use pipe as delimiter
	})
	f, err2 := os.Open(cli.Config.PathExclusionFile)
	if err2 != nil {
		return errors.New("Failed to open Blackout file")
	}
	defer f.Close()
	srvlog.Debug("Reading file ", cli.Config.PathExclusionFile)

	exclusionList := []*ExcludeDomainType{}
	gocsv.UnmarshalWithoutHeaders(f, &exclusionList)
	srvlog.Debug("Loaded these excluded domains : " + fmt.Sprintf("%#v", exclusionList))
	cli.Excludes = exclusionList

	return nil
}

func (cli *PsoftJmxClient) GetMetrics() ([]map[string]interface{}, error) {

	var requests []JMXQueryRequest

	err := cli.LoadTargets()
	if err != nil {
		return make([]map[string]interface{}, 0), err
	}
	srvlog.Debug("GetMetrics: Loaded these Targets : " + fmt.Sprintf("%#v", &cli.DomainList))
	_ = cli.LoadBlackouts()
	_ = cli.LoadExclusions()

	// setup a new pool of workers based on target domain list
	jmxpool := NewPoolManager(len(cli.DomainList), cli.Config.ConcurrentWorkers)
	srvlog.Debug("GetMetrics: Built pool for " + strconv.Itoa(cli.Config.ConcurrentWorkers))
	// build data for the jobs
	for i := 0; i < len(cli.DomainList); i++ {
		request := JMXQueryRequest{id: i}
		request.QueryList, err = cli.Attributes.BuildQueryStrings(cli.DomainList[i].DomainType)
		request.MetricsCfg = cli.Attributes.GetMetricConfig(cli.DomainList[i].DomainType)
		request.NGAddress = cli.Config.NailgunServerConn
		request.Blackouts = cli.Blackouts
		request.Excludes = cli.Excludes
		if err != nil {
			return make([]map[string]interface{}, 0), err
		}

		request.Target = *cli.DomainList[i]
		srvlog.Debug("GetMetrics: Added target: " + fmt.Sprintf("%#v", cli.DomainList[i]))
		srvlog.Debug("GetMetrics: Added Config: ") // + fmt.Sprintf("%#v", request.MetricsCfg))
		requests = append(requests, request)
	}

	// send the jobs to process and NailGun connection to the pool
	jmxresponse := jmxpool.RunJobs(requests)
	responseMetrics := make([]map[string]interface{}, 0)
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
