// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/UMN-PeopleSoft/nailgo"
	"net"
	"strings"
	"strconv"
)

var (
	jmxClass = "edu.umn.pssa.jmxquery.JMXQuery"
)

// defined the connection info to a JMX/rmi server
type JMXConnection struct {
	NGAddress  string
	ConnectURL string
	UserID     string
	Password   string
}

func (jmxConn *JMXConnection) RunJMXCommand(domainName string, attrList []string) (rawResponse string, err error) {
	rawResponse = ""
	ngBuf := new(bytes.Buffer)
	ngBufErr := new(bytes.Buffer)
	ngConn := &nailgo.NailgunConnection{}
	ngConn.Conn, err = jmxConn.GetNGConn()
	if err != nil {
		return rawResponse, err
	}
	ngConn.Output = ngBuf
	ngConn.Outerr = ngBufErr
	srvlog.Debug("JMX Conn: RunJMXCommand: " + fmt.Sprintf("%#v", ngConn))
	ngCmdArgs := []string{}
	ngCmdArgs = append(ngCmdArgs, "-url")
	ngCmdArgs = append(ngCmdArgs, jmxConn.ConnectURL)
	ngCmdArgs = append(ngCmdArgs, "-q")
	ngCmdArgs = append(ngCmdArgs, strings.Join(attrList, ";"))
	ngCmdArgs = append(ngCmdArgs, "-u")
	ngCmdArgs = append(ngCmdArgs, jmxConn.UserID)
	ngCmdArgs = append(ngCmdArgs, "-p")
	ngCmdArgs = append(ngCmdArgs, jmxConn.Password)

	srvlog.Debug("JMX Conn: ngConn.SendCommand: " + fmt.Sprintf("%#v", ngCmdArgs))
	exitCode, err := ngConn.SendCommand(jmxClass, ngCmdArgs)
	if exitCode != 0 {
		srvlog.Error("JMX ngConn.SendCommand error for " + domainName + ": " + strconv.Itoa(exitCode) + ":  response: " + ngBuf.String())
		if exitCode == 899 {
			return "", fmt.Errorf("Invalid user/password to access JMX target %s", domainName)
		} else {
			return "", fmt.Errorf("Unable to connect to JMX target %s, exitCode: %d response: %s", domainName, exitCode, ngBuf.String())
		}
	}
	srvlog.Debug("JMX Conn: ngConn.SendCommand: Completed sendcommand for " + domainName)
	rawResponse = ngBuf.String()
	return rawResponse, nil
}

func (jmxConn *JMXConnection) getNailGunStats() (rawResponse string, err error) {
	rawResponse = ""
	ngBuf := new(bytes.Buffer)
	ngConn := &nailgo.NailgunConnection{}
	ngConn.Conn, err = jmxConn.GetNGConn()
	if err != nil {
		return rawResponse, err
	}
	ngConn.Output = ngBuf

	exitCode, err2 := ngConn.SendCommand("ng-stats", []string{})
	if err2 != nil {
		return rawResponse, err2
	}
	if exitCode != 0 {
		return rawResponse, errors.New("Unable to get Nailgun Stats")
	}
	rawResponse = ngBuf.String()
	return rawResponse, nil

}

func (jmxConn *JMXConnection) GetNGConn() (net.Conn, error) {

	var err error
	var conn net.Conn

	if strings.HasPrefix(jmxConn.NGAddress, "local:") {
		socketFile := strings.Split(jmxConn.NGAddress, ":")[1]
		conn, err = net.Dial("unix", socketFile)
	} else {
		conn, err = net.Dial("tcp", jmxConn.NGAddress)
	}
	if err != nil {
		return nil, err
	}
	return conn, nil

}
