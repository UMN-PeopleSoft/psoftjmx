// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"sync"
	//"fmt"
)

// Response ties the request to the JMX metric results
type JMXResponse struct {
	JMXQueryRequest JMXQueryRequest
	MetricResults   map[string]interface{}
}

// structure to manage concurrent JMX queries to a pool of workers
type PoolManager struct {
	NumWorkers int
	NumJobs    int
	jobs       chan JMXQueryRequest
	results    chan JMXResponse
}

// Worker to process the JMX Request for each job/target.
func (p *PoolManager) jmxWorker(wg *sync.WaitGroup) {
	for job := range p.jobs {
		//srvlog.Info("Calling job.SendJMXRequest()") // with: " +  fmt.Sprintf("%#v", job))
		output := JMXResponse{job, job.SendJMXRequest()}
		p.results <- output
	}
	wg.Done()
}

// Setup the worker pools up to max number of workers
func (p *PoolManager) createJMXWorkerPool() {
	var wg sync.WaitGroup
	for i := 0; i < p.NumWorkers; i++ {
		wg.Add(1)
		go p.jmxWorker(&wg)
	}
	wg.Wait()
	close(p.results)
}

func (p *PoolManager) loadJMXRequests(jmxJobs []JMXQueryRequest) {
	for i := 0; i < len(jmxJobs); i++ {
		p.jobs <- jmxJobs[i]
	}
	close(p.jobs)

}

// Aggregate the metrics for all targets as they complete
func (p *PoolManager) waitForJMXResponse(metricDataChan chan []JMXResponse) {
	jmxData := []JMXResponse{}
	for result := range p.results {
		tmpResult := result
		//srvlog.Debug("JMX Pool: Captured JMX response: " + fmt.Sprintf("%#v", tmpResult))
		jmxData = append(jmxData, tmpResult)
	}
	metricDataChan <- jmxData
}

// Core concurrent generator based on # of targets
func (p *PoolManager) RunJobs(jmxJobs []JMXQueryRequest) []JMXResponse {

	metricDataChan := make(chan []JMXResponse)
	go p.loadJMXRequests(jmxJobs)
	go p.waitForJMXResponse(metricDataChan)
	// start workers
	srvlog.Debug("JMX Pool: Starting worker pool")
	p.createJMXWorkerPool()
	// wait until completed
	jmxresponse := <-metricDataChan

	return jmxresponse

}

func NewPoolManager(noOfJobs int, noOfWorkers int) *PoolManager {

	poolManager := &PoolManager{NumWorkers: noOfWorkers, NumJobs: noOfJobs}
	poolManager.jobs = make(chan JMXQueryRequest)
	poolManager.results = make(chan JMXResponse)
	return poolManager

}
