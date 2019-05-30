// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"fmt"
	"strings"
)

// Thread request payload of query attributes to search for and target domains
type JMXQueryRequest struct {
	id         int
	QueryList  []string
	MetricsCfg Metrics
	Target     PsoftDomain
	NGAddress  string
	Blackouts  []*BlackoutType      // list of domains or envs in a blackout
	Excludes   []*ExcludeDomainType // exiting domains to skip for monitoring, ie only used for peak loads

}

const (
	jmxTuxedoURLPrefix = "service:jmx:rmi://"
	jmxTuxedoURLPath   = "/DomainRuntime/DefaultConnector"
	jmxWebURLPrefix    = "service:jmx:t3://"
	jmxWebURLPath      = "/jndi/weblogic.management.mbeanservers.domainruntime"
)

func (j *JMXQueryRequest) inBlackout(target PsoftDomain) bool {
	for _, blackout := range j.Blackouts {
		appenv := ""
		if strings.Contains(blackout.DomainEnv, "ENV") {
			appenv = blackout.DomainEnv[strings.LastIndex(blackout.DomainEnv, "ENV"):]
		}
		if blackout.DomainEnv == target.DomainName || appenv == target.App+target.Env {
			return true
		}
	}
	return false
}

func (j *JMXQueryRequest) isExcluded(target PsoftDomain) bool {
	for _, exclude := range j.Excludes {
		if exclude.DomainName == target.DomainName {
			return true
		}
	}
	return false
}

// Main entry point for each threaded request to get metrics for a target
func (j *JMXQueryRequest) SendJMXRequest() map[string]interface{} {
	var url string
	var mappedResults map[string]interface{}

	// Check if the target is in blackout or excluded list, skip if so, but always return metric map
	if j.inBlackout(j.Target) {
		mappedResults = make(map[string]interface{})
		mappedResults["Status"] = "Blackout" // blackout
	} else if j.isExcluded(j.Target) {
		mappedResults = make(map[string]interface{})
		mappedResults["Status"] = "Excluded" // excluded
	} else {
		// Good to get metrics
		if j.Target.DomainType == "web" {
			url = jmxWebURLPrefix +
				j.Target.HostName +
				":" +
				j.Target.JMXPort +
				jmxWebURLPath
		} else {
			url = jmxTuxedoURLPrefix +
				j.Target.HostName +
				"/jndi/rmi://" +
				j.Target.HostName +
				":" +
				j.Target.JMXPort +
				"/" +
				j.Target.DomainName +
				jmxTuxedoURLPath
		}

		conn := &JMXConnection{
			NGAddress:  j.NGAddress,
			ConnectURL: url,
			UserID:     j.Target.JMXUser,
			Password:   j.Target.JMXPassword,
		}

		srvlog.Debug("JMX Request: SendJMXRequest for " + j.Target.DomainName + ": " + fmt.Sprintf("%#v", conn))

		// Make the JMX Query Call, return just the raw string
		jmxResponse, err := conn.RunJMXCommand(j.Target.DomainName, j.QueryList)
		if err != nil {
			srvlog.Error("JMX Request: RunJMXCommand Error response for " + j.Target.DomainName + " : " + jmxResponse + " error: " + err.Error())
			mappedResults = make(map[string]interface{})
			mappedResults["errorMsg"] = err.Error()
			if strings.Contains(err.Error(), "password") {
				mappedResults["status"] = "Config Error"
			} else {
				mappedResults["status"] = "Down"
			}
		} else {
			srvlog.Debug("JMX Request: RunJMXCommand response : " + jmxResponse)
			// Convert the results only if there are valid results
			mappedResults, err = j.MetricsCfg.MapData(j.Target.DomainType, jmxResponse)
			if err != nil {
				srvlog.Info("Failed to run MapData for %s: %s\n", j.Target.DomainName, err)
				mappedResults = make(map[string]interface{})
				mappedResults["errorMsg"] = err.Error()
				mappedResults["status"] = "Config Error" // config error
			} else {
				// valid target, valid results and map
				mappedResults["status"] = "Up" //up
			}
		}
	}
	// always add target attribues to the metric data to tie metrics (or errors) to a unique target
	mappedResults["domain_name"] = j.Target.DomainName
	mappedResults["domain_type"] = j.Target.DomainType
	mappedResults["purpose"] = j.Target.Purpose
	mappedResults["app"] = j.Target.App
	mappedResults["env"] = j.Target.Env
	mappedResults["appenv"] = j.Target.App + j.Target.Env
	mappedResults["serverName"] = j.Target.ServerName
	mappedResults["host"] = j.Target.HostName
	mappedResults["tools_version"] = j.Target.ToolsVer
	mappedResults["weblogic_ersion"] = j.Target.WeblogicVer
	//srvlog.Debug("Final Results: " + fmt.Sprintf("%#v", mappedResults))
	return mappedResults
}
