package main

import (
	"C"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
	"github.com/microsoft/ApplicationInsights-Go/appinsights"
	"github.com/mitchellh/mapstructure"
	"github.com/ugorji/go/codec"
)

// Taken from fluent-bit-go to modify
type FLBDecoder struct {
	handle *codec.MsgpackHandle
	mpdec  *codec.Decoder
}

type FLBTime struct {
	time.Time
}

func (f FLBTime) WriteExt(interface{}) []byte {
	panic("unsupported")
}

func (f FLBTime) ReadExt(i interface{}, b []byte) {
	out := i.(*FLBTime)
	sec := binary.BigEndian.Uint32(b)
	usec := binary.BigEndian.Uint32(b[4:])
	out.Time = time.Unix(int64(sec), int64(usec))
}

func (f FLBTime) ConvertExt(v interface{}) interface{} {
	return nil
}

func (f FLBTime) UpdateExt(dest interface{}, v interface{}) {
	panic("unsupported")
}

type AggregationType int64

const (
	UNSPECIFIED AggregationType = 0
	DELTA       AggregationType = 1
	CUMMULATIVE AggregationType = 2
)

func (at AggregationType) String() string {
	switch at {
	case UNSPECIFIED:
		return "unspecified"
	case DELTA:
		return "delta"
	case CUMMULATIVE:
		return "cumulative"
	default:
		return ""
	}
}

type MetricType int64

const (
	COUNTER   MetricType = 0
	GAUGE     MetricType = 1
	HISTOGRAM MetricType = 2
	SUMMARY   MetricType = 3
	UNTYPED   MetricType = 4
)

func (mt MetricType) String() string {
	switch mt {
	case COUNTER:
		return "counter"
	case GAUGE:
		return "gauge"
	case HISTOGRAM:
		return "histogram"
	case SUMMARY:
		return "summary"
	case UNTYPED:
		return "untyped"
	default:
		return ""
	}
}

type CMetrics struct {
	Meta struct {
		Cmetrics   map[string]interface{} `mapstructure:"cmetrics"`
		External   map[string]interface{} `mapstructure:"external"`
		Processing struct {
			StaticLabels []interface{} `mapstructure:"static_labels"`
		} `mapstructure:"processing"`
	} `mapstructure:"meta"`
	Metrics []struct {
		Meta struct {
			AggregationType AggregationType `mapstructure:"aggregation_type"`
			Labels          []string        `mapstructure:"labels"`
			/* Formatted full qualified metric name is: namespace_subsystem_name */
			Opts struct {
				Desc      string `mapstructure:"desc"`
				Name      string `mapstructure:"name"`
				Namespace string `mapstructure:"ns"`
				Subsystem string `mapstructure:"ss"`
			} `mapstructure:"opts"`
			Type MetricType `mapstructure:"type"`
			Ver  int        `mapstructure:"ver"`
		} `mapstructure:"meta"`
		Values []struct {
			Hash   int64    `mapstructure:"hash"`
			Labels []string `mapstructure:"labels"`
			Ts     int64    `mapstructure:"ts"`
			Value  float64  `mapstructure:"value"`
		} `mapstructure:"values"`
	} `mapstructure:"metrics"`
}

func (cm CMetrics) String() string {
	var ret strings.Builder

	for _, metric := range cm.Metrics {
		fullMetricName := fmt.Sprintf("%s_%s_%s", metric.Meta.Opts.Namespace, metric.Meta.Opts.Subsystem, metric.Meta.Opts.Name)
		ret.WriteString(fmt.Sprintf("# HELP %s %s\n", fullMetricName, metric.Meta.Opts.Desc))
		ret.WriteString(fmt.Sprintf("# TYPE %s %s\n", fullMetricName, metric.Meta.Type))

		for _, value := range metric.Values {
			ret.WriteString(fmt.Sprintf("%s{", fullMetricName))
			for i, labelName := range metric.Meta.Labels {
				ret.WriteString(fmt.Sprintf("%s=%s", labelName, value.Labels[i]))
				if i < len(metric.Meta.Labels)-1 {
					ret.WriteString(",")
				}
			}
			ret.WriteString(fmt.Sprintf("} %.0f\n", value.Value))
		}
	}

	return ret.String()
}

func SendPrometheusMetricsToAppInsights(records []map[interface{}]interface{}, tag string) int {
	telemetryPrefix := "prometheus"
	if tag == "prometheus.metrics.targetallocator" {
		telemetryPrefix = "target_allocator"
	}
	for _, record := range records {
		cMetrics := ConvertRecordToCMetrics(record)
		for _, metric := range cMetrics.Metrics {
			for _, value := range metric.Values {
				metricTelemetryItem := appinsights.NewMetricTelemetry(
					fmt.Sprintf("%s_%s_%s_%s", telemetryPrefix, metric.Meta.Opts.Namespace, metric.Meta.Opts.Subsystem, metric.Meta.Opts.Name),
					value.Value,
				)
				for i, labelName := range metric.Meta.Labels {
					metricTelemetryItem.Properties[labelName] = fmt.Sprintf("%s", value.Labels[i])
				}
				TelemetryClient.Track(metricTelemetryItem)
				Log(fmt.Sprintf("Sent telemetry for %s_%s_%s_%s", telemetryPrefix, metric.Meta.Opts.Namespace, metric.Meta.Opts.Subsystem, metric.Meta.Opts.Name))
			}
		}
	}
	return output.FLB_OK
}

func ConvertRecordToCMetrics(record map[interface{}]interface{}) (cMetrics CMetrics) {
	var result CMetrics
	mapstructure.WeakDecode(record, &result)
	return result
}

func NewDecoder(data unsafe.Pointer, length int) *FLBDecoder {
	var b []byte

	dec := new(FLBDecoder)
	dec.handle = new(codec.MsgpackHandle)
	dec.handle.SetBytesExt(reflect.TypeOf(FLBTime{}), 0, &FLBTime{})

	b = C.GoBytes(data, C.int(length))
	dec.mpdec = codec.NewDecoderBytes(b, dec.handle)

	return dec
}

func GetRecord(dec *FLBDecoder) (ret int, ts interface{}, rec map[interface{}]interface{}) {
	var check error
	var m interface{}

	check = dec.mpdec.Decode(&m)
	if check != nil {
		return -1, 0, nil
	}

	i := reflect.ValueOf(m)
	if i.Len() != 2 {
		return -2, 0, nil
	}

	switch i.Kind() {
	case reflect.Map: // Metrics
		map_data := i.Interface().(map[interface{}]interface{})
		return 0, 0, map_data
	case reflect.Slice: // Logs
		var t interface{}
		ts = i.Index(0).Interface()
		switch ty := ts.(type) {
		case FLBTime:
			t = ty
		case uint64:
			t = ty
		case []interface{}: // for Fluent Bit V2 metadata type of format
			s := reflect.ValueOf(ty)
			if s.Kind() != reflect.Slice || s.Len() < 2 {
				return -4, 0, nil
			}
			t = s.Index(0).Interface()
		default:
			return -5, 0, nil
		}
		data := i.Index(1)

		map_data, ok := data.Interface().(map[interface{}]interface{})
		if !ok {
			return -3, 0, nil
		}

		return 0, t, map_data

	default:
		return -2, 0, nil
	}
}
