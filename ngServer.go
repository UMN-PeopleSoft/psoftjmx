// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"github.com/UMN-PeopleSoft/nailgo"
	//"io"
	"os"
	"os/exec"
	"strings"
	//"bytes"
	"bufio"
	"errors"
	"fmt"
	"github.com/gosexy/to"
	"net"
	"time"
)

var (
	defaultNailgunServerConn = "local:/tmp/psmetric.socket"
)

const (
	nailGunServerJar     = "nailgun-server-1.0.0-SNAPSHOT-uber.jar"
	weblogicJMXClientJar = "wlthint3client.jar"
	jmxQueryJar          = "JMXQuery-1.0-SNAPSHOT.jar"
	nailGunClass         = "com.facebook.nailgun.NGServer"
	rmiResponseTimeoutMS = 20000
	ngServerLogConfig    = "logging.properties"
	ngServerLogFile      = "nailgun.log"
	ngHeartbeatTimeout   = 60000
	// socket tuning parms
	socketThreadPoolSize           = "50"
	threadPoolPercentSocketReaders = "80"
)

// Setup for starting the Nailgun server
type NailGunServer struct {
	JavaPath         string
	LogLevel         string
	TransportAddress string
	process          *os.Process
}

// Manages starting the Nailgun server for the JMX Query Client
func (ng *NailGunServer) StartNailgun() error {
	//clear old socket file if exists
	ng.removeSocket()
	//stop old nailgun server if running
	_, err := exec.Command("pkill -SIGKILL -f " + nailGunClass).Output()

	// setup the Nailgun Java Server logging config
	var logConfig []string
	logConfig = append(logConfig, "handlers = java.util.logging.FileHandler")
	logConfig = append(logConfig, ".level = "+ng.LogLevel)
	logConfig = append(logConfig, "java.util.logging.FileHandler.level = "+ng.LogLevel)
	logConfig = append(logConfig, "java.util.logging.FileHandler.limit = 10000000")
	logConfig = append(logConfig, "java.util.logging.FileHandler.pattern = logs/"+ngServerLogFile)
	logConfig = append(logConfig, "java.util.logging.FileHandler.count = 5")
	logConfig = append(logConfig, "java.util.logging.FileHandler.formatter = java.util.logging.SimpleFormatter")

	logFile, err := os.Create(ngServerLogConfig)
	if err != nil {
		return err
	}
	_, err = logFile.WriteString(strings.Join(logConfig, "\n"))
	logFile.Close()

	var classPath []string
	classPath = append(classPath, nailGunServerJar)
	classPath = append(classPath, weblogicJMXClientJar)
	classPath = append(classPath, jmxQueryJar)
	classPath = append(classPath, ".")

	var ngServerArgs []string
	ngServerArgs = append(ngServerArgs, "-Djna.nosys=true")
	ngServerArgs = append(ngServerArgs, "-Djava.util.logging.config.file="+ngServerLogConfig)
	ngServerArgs = append(ngServerArgs, "-Dsun.rmi.transport.tcp.responseTimeout="+to.String(rmiResponseTimeoutMS))
	// tuning the JMX client for socket connections, prevents error 402
	ngServerArgs = append(ngServerArgs, "-Dweblogic.ThreadPoolSize="+socketThreadPoolSize)
	ngServerArgs = append(ngServerArgs, "-Dweblogic.ThreadPoolPercentSocketReaders="+threadPoolPercentSocketReaders)

	ngServerArgs = append(ngServerArgs, "-classpath")
	ngServerArgs = append(ngServerArgs, strings.Join(classPath, ":"))
	ngServerArgs = append(ngServerArgs, "com.facebook.nailgun.NGServer")
	// parameters to the NailGun Server: listening address and timeout
	ngServerArgs = append(ngServerArgs, ng.TransportAddress)
	ngServerArgs = append(ngServerArgs, to.String(ngHeartbeatTimeout))

	cmd := exec.Command(ng.JavaPath + "/bin/java")
	cmd.Args = ngServerArgs
	// Pipe allows to read stdout while it's running
	//fmt.Println("Command is setup: " + ng.JavaPath + "/bin/java " + strings.Join(ngServerArgs, " "))
	stdoutReader, err := cmd.StdoutPipe()
	//fmt.Println("Started Reader")
	if err := cmd.Start(); err != nil {
		fmt.Println("Failed to start nailgun server")
		return err
	}
	//fmt.Println("Checking if it started")
	scanner := bufio.NewScanner(stdoutReader)
	time.Sleep(100 * time.Millisecond)

	for scanner.Scan() {
		//fmt.Println(scanner.Text())
		if strings.Contains(scanner.Text(), "started") {

			ng.process = cmd.Process
			return nil
		}
		if strings.Contains(scanner.Text(), "Nailgun server is not starting correctly") {
			return errors.New("Unable to start the Nailgun server")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (ng *NailGunServer) StopNailGun() error {

	var conn net.Conn
	var err error
	ngConn := &nailgo.NailgunConnection{}
	if strings.HasPrefix(ng.TransportAddress, "local:") {
		socketFile := strings.Split(ng.TransportAddress, ":")[1]
		conn, err = net.Dial("unix", socketFile)
	} else {
		conn, err = net.Dial("tcp", ng.TransportAddress)
	}
	if err != nil {
		return err
	}

	ngConn.Conn = conn

	exitCode, err := ngConn.SendCommand("ng-stop", []string{})
	if err != nil || exitCode != 0 {
		return err
	}
	// give it time to shutdown
	time.Sleep(200 * time.Millisecond)

	// make sure it is shutdown
	_ = ng.process.Kill()
	ng.removeSocket()
	return nil
}

func (ng *NailGunServer) removeSocket() {
	socketFile := strings.Split(ng.TransportAddress, ":")[1]
	_ = os.Remove(socketFile)
}
