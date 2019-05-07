// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"fmt"
)

// Thread request payload of query attributes to search for and target domains
type JMXQueryRequest struct {
	id         int
	QueryList  []string
	MetricsCfg Metrics
	Target     PsoftDomain
	NGAddress  string
}

const (
	jmxTuxedoURLPrefix = "service:jmx:rmi://"
	jmxTuxedoURLPath   = "/DomainRuntime/DefaultConnector"
	jmxWebURLPrefix    = "service:jmx:t3://"
	jmxWebURLPath      = "/jndi/weblogic.management.mbeanservers.domainruntime"
)

func (j *JMXQueryRequest) SendJMXRequest() map[string]string {
	var url string
	var mappedResults map[string]string

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
	srvlog.Debug("JMX Request: SendJMXRequest: " + fmt.Sprintf("%#v", conn))
	// make the JMX Call, return just the raw string
	jmxResponse, err := conn.RunJMXCommand(j.Target.DomainName, j.QueryList)
	if err != nil {
		srvlog.Error("JMX Request: RunJMXCommand Error response : " + jmxResponse + " error: " + err.Error())
		mappedResults = make(map[string]string)
		mappedResults["errorMsg"] = err.Error()
	} else {
		srvlog.Debug("JMX Request: RunJMXCommand response : " + jmxResponse)
		// Convert the results
		mappedResults, err = j.MetricsCfg.MapData(j.Target.DomainType, jmxResponse)
		if err != nil {
			fmt.Printf("Failed to run MapData: %s\n", err)
		}
	}
	// add target attribues to the metric data to tie metrics to a unique target
	mappedResults["domainName"] = j.Target.DomainName
	mappedResults["domainType"] = j.Target.DomainType
	mappedResults["purpose"] = j.Target.Purpose
	mappedResults["app"] = j.Target.App
	mappedResults["env"] = j.Target.Env
   mappedResults["appenv"] = j.Target.App + j.Target.Env
	mappedResults["serverName"] = j.Target.ServerName
	mappedResults["host"] = j.Target.HostName
	mappedResults["toolsVersion"] = j.Target.ToolsVer
	mappedResults["weblogicVersion"] = j.Target.WeblogicVer
	//srvlog.Debug("Final Results: " + fmt.Sprintf("%#v", mappedResults))
	return mappedResults
}
