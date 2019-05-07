// Poeplesoft Metric Capture via JMX
// TO_DO: YAML mapping

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
	MetricName  string `yaml:"metricName"`
	Role        string `yaml:"role"`
	AttrType    string `yaml:"attrType"`
	JMXClass    string `yaml:"jmxClass"`
	JMXAttrName string `yaml:"jmxAttrName"`
	JMXWhere    string `yaml:"attrWhere"`
}

type Metrics struct {
	Metrics []AttributeType `yaml:"metrics"`
}

// Structure to hold the JXM Query results
type JMXQueryResults struct {
	MBeanName     string `yaml:"mBeanName"`
	Attribute     string `yaml:"attribute"`
	AttributeType string `yaml:"attributeType"`
	Value         string `yaml:"value"`
}

// loaded JMX Attributes/Beans to do lookups/maps
type JMXAttributes struct {
	webJMXAttrib Metrics
	appJMXAttrib Metrics
	prcJMXAttrib Metrics
}

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
func (attr *JMXAttributes) BuildQueryStrings(targetType string) (queryList []string, err error) {

	var configList []AttributeType
	var matched int

	configList = attr.GetMetricConfig(targetType).Metrics

	for _, attr := range configList {
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

func (metricConfig *Metrics) MapData(targetType string, jmxDataString string) (map[string]string, error) {

	var mappedData = make(map[string]string)
	var jmxMapResults []JMXQueryResults

	// convert the json string to an array of JMXQueryResults struct
	err := yaml.Unmarshal([]byte(jmxDataString), &jmxMapResults)
	if err != nil {
		srvlog.Info("MapData: YAML Unmarshal failed for : " + jmxDataString + " error: " + err.Error())
		return nil, err
	}

	// main loop through each mapped metric
	for _, att := range metricConfig.Metrics {
		if att.AttrType == "sum" || att.AttrType == "avg" || att.AttrType == "max" || att.AttrType == "pct" {
			var sumValue float64
			var countValue float64
			var matchCount float64
			var maxValue float64
			var valueList = []float64{}
			// loop through each metric result to map to an attribute metric
			for _, metric := range jmxMapResults {
				if att.JMXAttrName == metric.Attribute {
					// we'll go a wildcard match to the actual bean
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
					mappedData[att.MetricName] = to.String(0.0)
				} else {
					mappedData[att.MetricName] = to.String(math.Round(sumValue/countValue*100) / 100)
				}
			} else if att.AttrType == "max" {
				mappedData[att.MetricName] = to.String(maxValue)
			} else if att.AttrType == "pct" {
				if matchCount == 0 {
					mappedData[att.MetricName] = to.String(0.0)
				} else {
					mappedData[att.MetricName] = to.String(math.Round(countValue/matchCount*100) / 100)
				}
			} else if att.AttrType == "stdev" {
				if len(valueList) > 0 {
					mappedData[att.MetricName] = to.String(math.Round(stat.StdDev(valueList, nil)*100) / 100)
				}
			} else if att.AttrType == "sum" {
				mappedData[att.MetricName] = to.String(sumValue)
			}
		} else if att.AttrType == "class" {
			for _, metric := range jmxMapResults {
				if att.JMXAttrName == metric.Attribute {
					//see if we can match bean name
					beanPattern := "ServerRuntime=PIA,"
					jmxClass := att.JMXClass
					if strings.Contains(metric.MBeanName, beanPattern) &&
						strings.Index(metric.MBeanName, beanPattern) < 12 {
						//bug: Weblogic jmx changes order of bean name, ServerRuntime moves to the front
						jmxClass := strings.Replace(jmxClass, beanPattern, "", -1)
						jmxClass = jmxClass[:8] + beanPattern + jmxClass[8:]
					}
					matched2, _ := filepath.Match(att.JMXClass, metric.MBeanName)
					matched3, _ := filepath.Match(jmxClass, metric.MBeanName)
					if matched2 || matched3 {
						if newInt, err2 := strconv.ParseInt(metric.Value, 10, 32); err2 == nil {
							mappedData[att.MetricName] = to.String(newInt)
						} else if newFloat, err3 := strconv.ParseFloat(metric.Value, 32); err3 == nil {
							mappedData[att.MetricName] = to.String(math.Round(newFloat*100) / 100)
						} else {
							mappedData[att.MetricName] = metric.Value
						}
						break
					}
				}
			}
		}
		if mappedData[att.MetricName] == "" {
			if att.JMXAttrName == "Health" || att.JMXAttrName == "HealthState" {
				mappedData[att.MetricName] = "Down"
			}
		}
	}
	if targetType == "app" && mappedData["Health"] != "Down" && to.Int64(mappedData["queue.server_count"]) > 0 {
      mappedData["load"] = to.String(to.Float64(mappedData["psappsrv_active_pct"]) + 
		                       math.Round(to.Float64(mappedData["queue.depth"]) / to.Float64(mappedData["queue.server_count"]) * 100) / 100)
	}

	return mappedData, nil

}
