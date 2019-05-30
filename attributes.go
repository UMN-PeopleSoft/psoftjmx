// Poeplesoft Metric Capture via JMX

package psoftjmx

import (
	"errors"
	"github.com/gonum/stat"
	"github.com/gosexy/to"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

type AttributeType struct {
	MetricName  string `yaml:"metricName"`  // file name to store the map metric value
	Role        string `yaml:"role"`        // psoft target type (web, app, scheduler)
	AttrType    string `yaml:"attrType"`    // raw value (class) or aggregated (by sum, max, pct, avg, stdev)
	JMXClass    string `yaml:"jmxClass"`    // JMX bean class name (path)
	JMXAttrName string `yaml:"jmxAttrName"` // metric attribute name
	JMXWhere    string `yaml:"attrWhere"`   // filter on aggregate sample
//	DataType    string `yaml:"dataType"`    // data type: int, string, float
//	Group       string `yaml:"group"`       // group to place field into for metric organization
}

// List of configured metrics from JMX source, pulled from a xxx_metric.yaml file
type Metrics struct {
	Metrics []AttributeType `yaml:"metrics"`
}

// Structure to hold the JXM Query results as formatted by the JMX Query Client
type JMXQueryResults struct {
	MBeanName     string `yaml:"mBeanName"`
	Attribute     string `yaml:"attribute"`
	AttributeType string `yaml:"attributeType"`
	Value         string `yaml:"value"`
}

// loaded JMX Attributes/Beans to do lookups/maps, will cache these from file
type JMXAttributes struct {
	webJMXAttrib Metrics
	appJMXAttrib Metrics
	prcJMXAttrib Metrics
}

// cache the metric configs from file
func (attr *JMXAttributes) GetAttributes(config *JMXConfig) error {
	var webConfig Metrics
	srcConfig, err := ioutil.ReadFile(config.AttribWebMetrics)
	if err != nil {
		return errors.New("Cant read file " + config.AttribWebMetrics)
	}
	srcBytes := []byte(srcConfig)
	err = yaml.Unmarshal(srcBytes, &webConfig)
	if err != nil {
		return errors.New("Cant unmarshal yaml file for Web Metric configs")
	}
	attr.webJMXAttrib = webConfig

	var appConfig Metrics
	srcConfig, err = ioutil.ReadFile(config.AttribAppMetrics)
	if err != nil {
		return errors.New("Cant read file " + config.AttribAppMetrics)
	}
	srcBytes = []byte(srcConfig)
	err = yaml.Unmarshal(srcBytes, &appConfig)
	if err != nil {
		return errors.New("Cant unmarshal yaml file for App Metric configs")
	}
	attr.appJMXAttrib = appConfig

	var prcConfig Metrics
	srcConfig, err = ioutil.ReadFile(config.AttribPrcMetrics)
	if err != nil {
		return errors.New("Cant read file " + config.AttribPrcMetrics)
	}
	srcBytes = []byte(srcConfig)
	err = yaml.Unmarshal(srcBytes, &prcConfig)
	if err != nil {
		return errors.New("Cant unmarshal yaml file for App Metric configs")
	}
	attr.prcJMXAttrib = prcConfig

	return nil
}

// pull back a cache config for a specific target type
func (attr *JMXAttributes) GetMetricConfig(targetType string) Metrics {
	var metricList Metrics
	if targetType == "web" {
		metricList = attr.webJMXAttrib
	} else if targetType == "app" {
		metricList = attr.appJMXAttrib
	} else if targetType == "prc" {
		metricList = attr.prcJMXAttrib
	}
	return metricList

}

// Generates the query strings for all of the JMX bean classes to be sent to the JMX Query Client
func (attr *JMXAttributes) BuildQueryStrings(targetType string) (queryList []string, err error) {

	var configList []AttributeType
	var matched int

	configList = attr.GetMetricConfig(targetType).Metrics

	for _, attr := range configList {
		// The query client uses a <bean-class>/<attribute>  format
		attLookupStr := attr.JMXClass + "/" + attr.JMXAttrName
		matched = 0
		for _, item := range queryList {
			if item == attLookupStr {
				matched = 1
				break
			}
		}
		if matched == 0 {
			queryList = append(queryList, attLookupStr)
		}
	}
	return queryList, nil
}

// Core metric mapping function to convert the raw metric data to a map
func (metricConfig *Metrics) MapData(targetType string, jmxDataString string) (map[string]interface{}, error) {

	var mappedData = make(map[string]interface{})
	var jmxMapResults []JMXQueryResults
	var strHealth string

	// convert the json string to an array of JMXQueryResults struct
	err := yaml.Unmarshal([]byte(jmxDataString), &jmxMapResults)
	if err != nil {
		srvlog.Info("MapData: YAML Unmarshal failed for : " + jmxDataString + " error: " + err.Error())
		return nil, err
	}

	// main loop through each configured known metric
	for _, att := range metricConfig.Metrics {
		if att.AttrType == "sum" || att.AttrType == "avg" || att.AttrType == "max" || att.AttrType == "pct" || att.AttrType == "stdev" {
			var sumValue float64
			var countValue float64
			var matchCount float64
			var maxValue float64
			var valueList = []float64{}
			// loop through each metric result mapping it to an configured attribute
			for _, metric := range jmxMapResults {
				if att.JMXAttrName == metric.Attribute {
					// we'll use a wildcard match to the actual bean since the class name can unclude * wildcard
					if matched, _ := filepath.Match(att.JMXClass, metric.MBeanName); matched {
						if att.AttrType == "pct" {
							matchCount++
						}
						if len(att.JMXWhere) > 1 {
							if strings.HasPrefix(att.JMXWhere, "!") {
								if strings.TrimLeft(att.JMXWhere, "!") == metric.Value {
									continue
								}
							} else {
								if att.JMXWhere != metric.Value {
									continue
								}
							}
						}
						if att.AttrType == "max" {
							if maxValue < to.Float64(metric.Value) {
								maxValue = to.Float64(metric.Value)
							}
						} else if att.AttrType == "stdev" {
							valueList = append(valueList, to.Float64(metric.Value))
						} else if att.AttrType == "sum" || att.AttrType == "avg" {
							if _, err1 := strconv.ParseFloat(metric.Value, 64); err1 == nil {
								sumValue = sumValue + to.Float64(metric.Value)
							} else {
								sumValue++
							}

						}
					}
					countValue++
				}
			}
			if att.AttrType == "avg" {
				if countValue == 0 {
					mappedData[att.MetricName] = 0.0
				} else {
					mappedData[att.MetricName] = math.Round(sumValue/countValue*100) / 100
				}
			} else if att.AttrType == "max" {
				mappedData[att.MetricName] = maxValue
			} else if att.AttrType == "pct" {
				if matchCount == 0 {
					mappedData[att.MetricName] = 0.0
				} else {
					mappedData[att.MetricName] = math.Round(countValue/matchCount*100) / 100
				}
			} else if att.AttrType == "stdev" {
				if len(valueList) > 0 {
					mappedData[att.MetricName] = math.Round(stat.StdDev(valueList, nil)*100) / 100
				}
			} else if att.AttrType == "sum" {
				mappedData[att.MetricName] = sumValue
			}
		} else if att.AttrType == "class" {
			for _, metric := range jmxMapResults {
				if att.JMXAttrName == metric.Attribute {
					//see if we can match bean name
					beanPattern := "ServerRuntime=PIA,"
					jmxClass := att.JMXClass
					if strings.Contains(metric.MBeanName, beanPattern) &&
						strings.Index(metric.MBeanName, beanPattern) < 12 {
						//bug: Weblogic jmx versions change order of bean name, ServerRuntime moves to the front
						jmxClass := strings.Replace(jmxClass, beanPattern, "", -1)
						jmxClass = jmxClass[:8] + beanPattern + jmxClass[8:]
					}
					matched2, _ := filepath.Match(att.JMXClass, metric.MBeanName)
					matched3, _ := filepath.Match(jmxClass, metric.MBeanName)
					if matched2 || matched3 {
						if att.JMXAttrName == "Health" && metric.Value == "" {
							mappedData[att.MetricName] = "Unavailable"
						} else if att.JMXAttrName == "HealthState" {
							switch {
							case strings.Contains(metric.Value, "HEALTH_OK"):
								strHealth = "Ok"
							case strings.Contains(metric.Value, "HEALTH_WARN"):
								strHealth = "Warning"
							case strings.Contains(metric.Value, "HEALTH_CRITICAL"):
								strHealth = "Critical"
							case strings.Contains(metric.Value, "HEALTH_FAILED"):
								strHealth = "Failed"
							case strings.Contains(metric.Value, "HEALTH_OVERLOADED"):
								strHealth = "Overloaded"
							default: // down, unavailable
								strHealth = "Unavailable"
							}
							mappedData[att.MetricName] = strHealth
						} else {
							if newInt, err2 := strconv.ParseInt(metric.Value, 10, 32); err2 == nil {
								mappedData[att.MetricName] = newInt
							} else if newFloat, err3 := strconv.ParseFloat(metric.Value, 32); err3 == nil {
								mappedData[att.MetricName] = math.Round(newFloat*100) / 100
							} else {
								mappedData[att.MetricName] = metric.Value
							}
						}
						break
					}
				}
			}
		}
		if mappedData[att.MetricName] == "" {
			if att.JMXAttrName == "Health" || att.JMXAttrName == "HealthState" {
				mappedData[att.MetricName] = "Unavailable"
			}
		}
	}
	// Build the new app server domain Load metric
	if targetType == "app" && mappedData["Health"] != 5 && mappedData["appsrv.queue.server_count"].(float64) > 0 {
		mappedData["appsrv.load"] = mappedData["appsrv.active_pct"].(float64) + math.Round(to.Float64(mappedData["appsrv.queue.depth"])/to.Float64(mappedData["appsrv.queue.server_count"])*100)/100
	}

	return mappedData, nil

}
