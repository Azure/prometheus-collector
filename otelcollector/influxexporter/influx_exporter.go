// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package influxexporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	//"math/rand"
	"time"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	//"github.com/influxdata/influxdb-client-go/v2"
	//"github.com/influxdata/influxdb-client-go/v2/api/write"

	"github.com/influxdata/line-protocol"
)

var measurementName string = strings.TrimSpace(os.Getenv("METRICS_NAMESPACE"))

func init() {
	if !(len(measurementName) > 0) {
		measurementName = measurementNameDefault
	}
	Log(fmt.Sprintf("Using measurement name as '%s'", measurementName))
}

type logDataBuffer struct {
	str strings.Builder
}

// Implement influx lp metric interface
type Metric struct {
	name   				string
	tags   				[]*protocol.Tag
	fields 				[]*protocol.Field
	timestamp     		time.Time
}

func (m Metric) Name() string {
	return m.name
}

func (m Metric) Time() time.Time {
	return m.timestamp
}

func (m Metric) FieldList() []*protocol.Field {
	return m.fields
}

func(m Metric) TagList() []*protocol.Tag {
	return m.tags
}

func (b *logDataBuffer) logEntry(format string, a ...interface{}) {
	b.str.WriteString(fmt.Sprintf(format, a...))
	b.str.WriteString("\n")
}

func (b *logDataBuffer) logAttr(label string, value string) {
	b.logEntry("    %-15s: %s", label, value)
}

func (b *logDataBuffer) logAttributeMap(label string, am pdata.AttributeMap) {
	if am.Len() == 0 {
		return
	}

	b.logEntry("%s:", label)
	am.ForEach(func(k string, v pdata.AttributeValue) {
		b.logEntry("     -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
	})
}

func (b *logDataBuffer) logStringMap(description string, sm pdata.StringMap) {
	if sm.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)
	sm.ForEach(func(k string, v string) {
		b.logEntry("     -> %s: %s", k, v)
	})
}

func (b *logDataBuffer) logInstrumentationLibrary(il pdata.InstrumentationLibrary) {
	b.logEntry(
		"InstrumentationLibrary %s %s",
		il.Name(),
		il.Version())
}

func (b *logDataBuffer) logMetricDescriptor(md pdata.Metric) {
	b.logEntry("Descriptor:")
	b.logEntry("     -> Name: %s", md.Name())
	b.logEntry("     -> Description: %s", md.Description())
	b.logEntry("     -> Unit: %s", md.Unit())
	b.logEntry("     -> DataType: %s", md.DataType().String())
}

func (b *logDataBuffer) logMetricDataPoints(m pdata.Metric) {
	switch m.DataType() {
	case pdata.MetricDataTypeNone:
		return
	case pdata.MetricDataTypeIntGauge:
		b.logIntDataPoints(m.IntGauge().DataPoints())
		//logIntDataPointsInflux()
	case pdata.MetricDataTypeDoubleGauge:
		b.logDoubleDataPoints(m.DoubleGauge().DataPoints())
		//logIntDataPointsInflux()
	case pdata.MetricDataTypeIntSum:
		data := m.IntSum()
		b.logEntry("     -> IsMonotonic: %t", data.IsMonotonic())
		b.logEntry("     -> AggregationTemporality: %s", data.AggregationTemporality().String())
		b.logIntDataPoints(data.DataPoints())
	case pdata.MetricDataTypeDoubleSum:
		data := m.DoubleSum()
		b.logEntry("     -> IsMonotonic: %t", data.IsMonotonic())
		b.logEntry("     -> AggregationTemporality: %s", data.AggregationTemporality().String())
		b.logDoubleDataPoints(data.DataPoints())
	case pdata.MetricDataTypeIntHistogram:
		data := m.IntHistogram()
		b.logEntry("     -> AggregationTemporality: %s", data.AggregationTemporality().String())
		b.logIntHistogramDataPoints(data.DataPoints())
	case pdata.MetricDataTypeDoubleHistogram:
		data := m.DoubleHistogram()
		b.logEntry("     -> AggregationTemporality: %s", data.AggregationTemporality().String())
		b.logDoubleHistogramDataPoints(data.DataPoints())
	case pdata.MetricDataTypeDoubleSummary:
		data := m.DoubleSummary()
		b.logDoubleSummaryDataPoints(data.DataPoints())
	}
}

func (b *logDataBuffer) logIntDataPoints(ps pdata.IntDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("IntDataPoints #%d", i)
		b.logDataPointLabels(p.LabelsMap())

		b.logEntry("StartTime: %d", p.StartTime())
		b.logEntry("Timestamp: %d", p.Timestamp())
		b.logEntry("Value: %d", p.Value())
	}
}

/*func logIntDataPointsInflux() {
	// Create a new client using an InfluxDB server base URL and an authentication token
	client := influxdb2.NewClient("tcp://localhost:8089", "")
	// Get non-blocking write client
	writeAPI := client.WriteAPI("my-org", "my-bucket")
	// write some points
	for i := 0; i < 100; i++ {
		// create point
		p := write.NewPoint(
			"system",
			map[string]string{
				"id":       fmt.Sprintf("rack_%v", i%10),
				"vendor":   "AWS",
				"hostname": fmt.Sprintf("host_%v", i%100),
			},
			map[string]interface{}{
				"temperature": rand.Float64() * 80.0,
				"disk_free":   rand.Float64() * 1000.0,
				"disk_total":  (i/10 + 1) * 1000000,
				"mem_total":   (i/100 + 1) * 10000000,
				"mem_free":    rand.Uint64(),
			},
			time.Now())
		// write asynchronously
		writeAPI.WritePoint(p)
	}
	// Force all unwritten data to be sent
	writeAPI.Flush()
	// Ensures background processes finishes
	client.Close()
}*/

func (b *logDataBuffer) logDoubleDataPoints(ps pdata.DoubleDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("DoubleDataPoints #%d", i)
		b.logDataPointLabels(p.LabelsMap())

		b.logEntry("StartTime: %d", p.StartTime())
		b.logEntry("Timestamp: %d", p.Timestamp())
		b.logEntry("Value: %f", p.Value())
	}
}

func (b *logDataBuffer) logDoubleHistogramDataPoints(ps pdata.DoubleHistogramDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("HistogramDataPoints #%d", i)
		b.logDataPointLabels(p.LabelsMap())

		b.logEntry("StartTime: %d", p.StartTime())
		b.logEntry("Timestamp: %d", p.Timestamp())
		b.logEntry("Count: %d", p.Count())
		b.logEntry("Sum: %f", p.Sum())

		bounds := p.ExplicitBounds()
		if len(bounds) != 0 {
			for i, bound := range bounds {
				b.logEntry("ExplicitBounds #%d: %f", i, bound)
			}
		}

		buckets := p.BucketCounts()
		if len(buckets) != 0 {
			for j, bucket := range buckets {
				b.logEntry("Buckets #%d, Count: %d", j, bucket)
			}
		}
	}
}

func (b *logDataBuffer) logIntHistogramDataPoints(ps pdata.IntHistogramDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("HistogramDataPoints #%d", i)
		b.logDataPointLabels(p.LabelsMap())

		b.logEntry("StartTime: %d", p.StartTime())
		b.logEntry("Timestamp: %d", p.Timestamp())
		b.logEntry("Count: %d", p.Count())
		b.logEntry("Sum: %d", p.Sum())

		bounds := p.ExplicitBounds()
		if len(bounds) != 0 {
			for i, bound := range bounds {
				b.logEntry("ExplicitBounds #%d: %f", i, bound)
			}
		}

		buckets := p.BucketCounts()
		if len(buckets) != 0 {
			for j, bucket := range buckets {
				b.logEntry("Buckets #%d, Count: %d", j, bucket)
			}
		}
	}
}

func (b *logDataBuffer) logDoubleSummaryDataPoints(ps pdata.DoubleSummaryDataPointSlice) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		b.logEntry("SummaryDataPoints #%d", i)
		b.logDataPointLabels(p.LabelsMap())

		b.logEntry("StartTime: %d", p.StartTime())
		b.logEntry("Timestamp: %d", p.Timestamp())
		b.logEntry("Count: %d", p.Count())
		b.logEntry("Sum: %f", p.Sum())

		quantiles := p.QuantileValues()
		for i := 0; i < quantiles.Len(); i++ {
			quantile := quantiles.At(i)
			b.logEntry("QuantileValue #%d: Quantile %f, Value %f", i, quantile.Quantile(), quantile.Value())
		}
	}
}

func (b *logDataBuffer) logDataPointLabels(labels pdata.StringMap) {
	b.logStringMap("Data point labels", labels)
}

func (b *logDataBuffer) logLogRecord(lr pdata.LogRecord) {
	b.logEntry("Timestamp: %d", lr.Timestamp())
	b.logEntry("Severity: %s", lr.SeverityText())
	b.logEntry("ShortName: %s", lr.Name())
	b.logEntry("Body: %s", attributeValueToString(lr.Body()))
	b.logAttributeMap("Attributes", lr.Attributes())
}

func (b *logDataBuffer) logEvents(description string, se pdata.SpanEventSlice) {
	if se.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)
	for i := 0; i < se.Len(); i++ {
		e := se.At(i)
		b.logEntry("SpanEvent #%d", i)
		b.logEntry("     -> Name: %s", e.Name())
		b.logEntry("     -> Timestamp: %d", e.Timestamp())
		b.logEntry("     -> DroppedAttributesCount: %d", e.DroppedAttributesCount())

		if e.Attributes().Len() == 0 {
			return
		}
		b.logEntry("     -> Attributes:")
		e.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			b.logEntry("         -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
		})
	}
}

func (b *logDataBuffer) logLinks(description string, sl pdata.SpanLinkSlice) {
	if sl.Len() == 0 {
		return
	}

	b.logEntry("%s:", description)

	for i := 0; i < sl.Len(); i++ {
		l := sl.At(i)
		b.logEntry("SpanLink #%d", i)
		b.logEntry("     -> Trace ID: %s", l.TraceID().HexString())
		b.logEntry("     -> ID: %s", l.SpanID().HexString())
		b.logEntry("     -> TraceState: %s", l.TraceState())
		b.logEntry("     -> DroppedAttributesCount: %d", l.DroppedAttributesCount())
		if l.Attributes().Len() == 0 {
			return
		}
		b.logEntry("     -> Attributes:")
		l.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			b.logEntry("         -> %s: %s(%s)", k, v.Type().String(), attributeValueToString(v))
		})
	}
}

func attributeValueToString(av pdata.AttributeValue) string {
	switch av.Type() {
	case pdata.AttributeValueSTRING:
		return av.StringVal()
	case pdata.AttributeValueBOOL:
		return strconv.FormatBool(av.BoolVal())
	case pdata.AttributeValueDOUBLE:
		return strconv.FormatFloat(av.DoubleVal(), 'f', -1, 64)
	case pdata.AttributeValueINT:
		return strconv.FormatInt(av.IntVal(), 10)
	case pdata.AttributeValueARRAY:
		return attributeValueArrayToString(av.ArrayVal())
	default:
		return fmt.Sprintf("<Unknown OpenTelemetry attribute value type %q>", av.Type())
	}
}

func attributeValueArrayToString(av pdata.AnyValueArray) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < av.Len(); i++ {
		if i < av.Len()-1 {
			fmt.Fprintf(&b, "%s, ", attributeValueToString(av.At(i)))
		} else {
			b.WriteString(attributeValueToString(av.At(i)))
		}
	}

	b.WriteByte(']')
	return b.String()
}

type influxExporter struct {
	logger *zap.Logger
	debug  bool
}

func (s *influxExporter) pushTraceData(
	_ context.Context,
	td pdata.Traces,
) (int, error) {

	s.logger.Info("TracesExporter", zap.Int("#spans", td.SpanCount()))

	if !s.debug {
		return 0, nil
	}

	buf := logDataBuffer{}
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		buf.logEntry("ResourceSpans #%d", i)
		rs := rss.At(i)
		buf.logAttributeMap("Resource labels", rs.Resource().Attributes())
		ilss := rs.InstrumentationLibrarySpans()
		for j := 0; j < ilss.Len(); j++ {
			buf.logEntry("InstrumentationLibrarySpans #%d", j)
			ils := ilss.At(j)
			buf.logInstrumentationLibrary(ils.InstrumentationLibrary())

			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				buf.logEntry("Span #%d", k)
				span := spans.At(k)
				buf.logAttr("Trace ID", span.TraceID().HexString())
				buf.logAttr("Parent ID", span.ParentSpanID().HexString())
				buf.logAttr("ID", span.SpanID().HexString())
				buf.logAttr("Name", span.Name())
				buf.logAttr("Kind", span.Kind().String())
				buf.logAttr("Start time", span.StartTime().String())
				buf.logAttr("End time", span.EndTime().String())

				buf.logAttr("Status code", span.Status().Code().String())
				buf.logAttr("Status message", span.Status().Message())

				buf.logAttributeMap("Attributes", span.Attributes())
				buf.logEvents("Events", span.Events())
				buf.logLinks("Links", span.Links())
			}
		}
	}
	s.logger.Debug(buf.str.String())

	return 0, nil
}

func sendToME(metric *Metric) {
	buf := &bytes.Buffer{}
	serializer := protocol.NewEncoder(buf)
	serializer.SetMaxLineBytes(1024)
	serializer.Encode(metric)
	Log(fmt.Sprintf(buf.String()))
	bytesWritten, er := Write2ME(buf.Bytes())
	if er == nil {
		Log(fmt.Sprintf("Successfully wrote %d bytes to ME", bytesWritten) )
	} else {
		Log(fmt.Sprintf("Error writing metric to ME : %s", er.Error()))
	}
}

func convertTypeDoubleToInflux(ps pdata.DoubleDataPointSlice, name string) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)

		timestamp := time.Unix(0, int64(p.Timestamp()))

		var labels []*protocol.Tag
		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			labels = append(labels, label)
		})

		var fields []*protocol.Field
		field := &protocol.Field {
			Key: name, Value: float64(p.Value()),
		}
		fields = append(fields, field)
	
		metric := &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: fields,
		}

		sendToME(metric)
	}
}

func convertTypeIntToInflux(ps pdata.IntDataPointSlice, name string) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)

		timestamp := time.Unix(0, int64(p.Timestamp()))

		var labels []*protocol.Tag
		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			labels = append(labels, label)
		})

		var fields []*protocol.Field
		field := &protocol.Field {
			Key: name, Value: float64(p.Value()),
		}
		fields = append(fields, field)
	
		metric := &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: fields,
		}

		sendToME(metric)
	}
}

func convertTypeDoubleSummaryToInflux(ps pdata.DoubleSummaryDataPointSlice, name string) {
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		timestamp := time.Unix(0, int64(p.Timestamp()))

		var labels []*protocol.Tag
		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			labels = append(labels, label)
		})

		// Series for count
		var fields []*protocol.Field
	  countField := &protocol.Field {
			Key: fmt.Sprintf("%s_count", name), Value: float64(p.Count()),
		}
		fields = append(fields, countField)
		metric := &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: fields,
		}
		sendToME(metric)

		// Series for sum
		fields = fields[:0]
		totalField := &protocol.Field {
			Key: fmt.Sprintf("%s_sum", name), Value: float64(p.Sum()),
		}
		fields = append(fields, totalField)
		metric = &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: fields,
		}
		sendToME(metric)

		// Series for each quantile
		quantiles := p.QuantileValues()
		for i := 0; i < quantiles.Len(); i++ {

			// Remove last quantile label
			if i > 0 && len(labels) > 0 {
				labels = labels[:len(labels)-1]
			}

			quantile := quantiles.At(i)
			label := &protocol.Tag{Key: "quantile", Value: fmt.Sprintf("%f", quantile.Quantile())}
			labels = append(labels, label)

			fields = fields[:0]
			field := &protocol.Field {
				Key: fmt.Sprintf(name), Value: float64(quantile.Value()),
			}
			fields = append(fields, field)
			
			metric = &Metric{
				timestamp: timestamp, name: measurementName, tags: labels, fields: fields,
			}
			sendToME(metric)
		}
	}
}

func convertDoubleHistogramToInflux(ps pdata.DoubleHistogramDataPointSlice, name string) {
	var metrics []*Metric
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		timestamp := time.Unix(0, int64(p.Timestamp()))

		var labels []*protocol.Tag
		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			labels = append(labels, label)
		})

		var sumCountFields []*protocol.Field
		
		//add sum, count series
		sumField := &protocol.Field {
			Key: name + "_sum", Value: float64(p.Sum()),
		}
		countField := &protocol.Field {
			Key: name + "_count", Value: float64(p.Count()),
		}
		
		sumCountFields = append(sumCountFields, sumField, countField)
	
		sumCountMetrics := &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: sumCountFields,
		}

		metrics = append(metrics,sumCountMetrics)

		//add +Inf series
		var infSerieslabels []*protocol.Tag
		var infSeriesFields []*protocol.Field
		var infSeriesMetric *Metric
		
		infSeriesFields = append(infSeriesFields, &protocol.Field{ 
		Key: name + "_bucket", Value: float64(p.Count())} )

		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			infSerieslabels = append(infSerieslabels, label)
		})
		infSerieslabels = append(infSerieslabels, &protocol.Tag{Key: "le", Value: fmt.Sprint("+Inf")})
		infSeriesMetric = &Metric{
			timestamp: timestamp, name: measurementName, tags: infSerieslabels, fields: infSeriesFields,
		}
		metrics = append(metrics,infSeriesMetric)

		//add all explicit le series
		
		for index, _ :=range p.ExplicitBounds() {
			var leSerieslabels []*protocol.Tag
			
			var leSeriesMetric *Metric
			

			var cumulativeValue uint64
			if index >= len(p.BucketCounts()) { //take only explicit bounds
				break
			}
			for bucketIndex, _ := range p.BucketCounts() {
				if bucketIndex > index {
					break
				}
				cumulativeValue +=  p.BucketCounts()[bucketIndex] //bucketValue
			}
			var fields []*protocol.Field
			fields = append(fields, &protocol.Field{ 
			Key: name + "_bucket", Value: float64(cumulativeValue)} )
			
			//var leSerieslabels []*protocol.Tag
			p.LabelsMap().ForEach(func(k string, v string) {
				label := &protocol.Tag{Key: k, Value: v}
				leSerieslabels = append(leSerieslabels, label)
			})

			leSerieslabels = append(leSerieslabels, &protocol.Tag{Key: "le", Value: strconv.FormatFloat(p.ExplicitBounds()[index], 'f',-1, 64)})
			//Value: strconv.FormatFloat(p.ExplicitBounds()[index], 'f',-1, 64)})//(bound, 'f',-1, 64)})
			leSeriesMetric = &Metric{
				timestamp: timestamp, name: measurementName, tags: leSerieslabels, fields: fields,
			}
			metrics = append(metrics,leSeriesMetric)
		}
	}
	for _, metric:= range metrics {
		sendToME(metric)
	}

}

func convertIntHistogramToInflux(ps pdata.IntHistogramDataPointSlice, name string) {
	var metrics []*Metric
	for i := 0; i < ps.Len(); i++ {
		p := ps.At(i)
		timestamp := time.Unix(0, int64(p.Timestamp()))

		var labels []*protocol.Tag
		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			labels = append(labels, label)
		})

		var sumCountFields []*protocol.Field
		
		//add sum, count series
		sumField := &protocol.Field {
			Key: name + "_sum", Value: float64(p.Sum()),
		}
		countField := &protocol.Field {
			Key: name + "_count", Value: float64(p.Count()),
		}
		
		sumCountFields = append(sumCountFields, sumField, countField)
	
		sumCountMetrics := &Metric{
			timestamp: timestamp, name: measurementName, tags: labels, fields: sumCountFields,
		}

		metrics = append(metrics,sumCountMetrics)

		//add +Inf series
		var infSerieslabels []*protocol.Tag
		var infSeriesFields []*protocol.Field
		var infSeriesMetric *Metric
		
		infSeriesFields = append(infSeriesFields, &protocol.Field{ 
		Key: name + "_bucket", Value: float64(p.Count())} )

		p.LabelsMap().ForEach(func(k string, v string) {
			label := &protocol.Tag{Key: k, Value: v}
			infSerieslabels = append(infSerieslabels, label)
		})
		infSerieslabels = append(infSerieslabels, &protocol.Tag{Key: "le", Value: fmt.Sprint("+Inf")})
		infSeriesMetric = &Metric{
			timestamp: timestamp, name: measurementName, tags: infSerieslabels, fields: infSeriesFields,
		}
		metrics = append(metrics,infSeriesMetric)

		//add all explicit le series
		
		for index, _ :=range p.ExplicitBounds() {
			var leSerieslabels []*protocol.Tag
			
			var leSeriesMetric *Metric
			

			var cumulativeValue uint64
			if index >= len(p.BucketCounts()) { //take only explicit bounds
				break
			}
			for bucketIndex, _ := range p.BucketCounts() {
				if bucketIndex > index {
					break
				}
				cumulativeValue +=  p.BucketCounts()[bucketIndex] //bucketValue
			}
			var fields []*protocol.Field
			fields = append(fields, &protocol.Field{ 
			Key: name + "_bucket", Value: float64(cumulativeValue)} )
			
			//var leSerieslabels []*protocol.Tag
			p.LabelsMap().ForEach(func(k string, v string) {
				label := &protocol.Tag{Key: k, Value: v}
				leSerieslabels = append(leSerieslabels, label)
			})

			leSerieslabels = append(leSerieslabels, &protocol.Tag{Key: "le", Value: fmt.Sprint(p.ExplicitBounds()[index])})
			//Value: strconv.FormatFloat(p.ExplicitBounds()[index], 'f',-1, 64)})//(bound, 'f',-1, 64)})
			leSeriesMetric = &Metric{
				timestamp: timestamp, name: measurementName, tags: leSerieslabels, fields: fields,
			}
			metrics = append(metrics,leSeriesMetric)
		}
	}
	for _, metric:= range metrics {
		sendToME(metric)
	}

}

func (b *logDataBuffer) convertMetricToInflux(m pdata.Metric, s *influxExporter) {
	var name = m.Name()

	switch m.DataType() {
	case pdata.MetricDataTypeIntGauge:
		ps := m.IntGauge().DataPoints()
		convertTypeIntToInflux(ps, name)
	case pdata.MetricDataTypeDoubleGauge:
		ps := m.DoubleGauge().DataPoints()
		convertTypeDoubleToInflux(ps, name)
	case pdata.MetricDataTypeIntSum:
		data := m.IntSum()
		if data.IsMonotonic() && data.AggregationTemporality() == pdata.AggregationTemporalityCumulative {
			ps := data.DataPoints()
			convertTypeIntToInflux(ps, name)
		}
	case pdata.MetricDataTypeDoubleSum:
		data := m.DoubleSum()
		if data.IsMonotonic() && data.AggregationTemporality() == pdata.AggregationTemporalityCumulative {
			ps := data.DataPoints()
			convertTypeDoubleToInflux(ps, name)
		}
	case pdata.MetricDataTypeDoubleSummary:
		ps := m.DoubleSummary().DataPoints()
		convertTypeDoubleSummaryToInflux(ps, name)
	case pdata.MetricDataTypeDoubleHistogram:
		ps := m.DoubleHistogram().DataPoints()
		convertDoubleHistogramToInflux(ps, name)
	case pdata.MetricDataTypeIntHistogram:
		ps := m.IntHistogram().DataPoints()
		convertIntHistogramToInflux(ps, name)
	default:
		return
	}

	/*if metric != nil {
		buf := &bytes.Buffer{}
		serializer := protocol.NewEncoder(buf)
		serializer.SetMaxLineBytes(1024)
		serializer.Encode(metric)
		bytesWritten, er := Write2ME(buf.Bytes())
		if er != nil {
			fmt.Println("Successfully wrote %d bytes to ME", bytesWritten )
		} else {
			fmt.Println("Error writing metric to ME %s", er.Error())
		}
	} else {
		fmt.Println("Empty metric !!! Something is not correct!")
	}*/

}

func (s *influxExporter) pushMetricsData(
	_ context.Context,
	md pdata.Metrics,
) (int, error) {
	s.logger.Info("MetricsExporter", zap.Int("#metrics", md.MetricCount()))

	//if !s.debug {
		//return 0, nil
	//}

	buf := logDataBuffer{}
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		//buf.logEntry("ResourceMetrics #%d", i)
		rm := rms.At(i)
		//buf.logAttributeMap("Resource labels", rm.Resource().Attributes())
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			//buf.logEntry("InstrumentationLibraryMetrics #%d", j)
			ilm := ilms.At(j)
			//buf.logInstrumentationLibrary(ilm.InstrumentationLibrary())
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				//buf.logEntry("Metric #%d", k)
				metric := metrics.At(k)
				buf.convertMetricToInflux(metric, s)
				//buf.logMetricDescriptor(metric)
				//buf.logMetricDataPoints(metric)
			}
		}
	}

	return 0, nil
}

// newTraceExporter creates an exporter.TracesExporter that just drops the
// received data and logs debugging messages.
func newTraceExporter(config configmodels.Exporter, level string, logger *zap.Logger) (component.TracesExporter, error) {
	s := &influxExporter{
		debug:  strings.ToLower(level) == "debug",
		logger: logger,
	}

	return exporterhelper.NewTraceExporter(
		config,
		logger,
		s.pushTraceData,
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(logger)),
	)
}

// newMetricsExporter creates an exporter.MetricsExporter that just drops the
// received data and logs debugging messages.
func newMetricsExporter(config configmodels.Exporter, level string, logger *zap.Logger) (component.MetricsExporter, error) {
	s := &influxExporter{
		debug:  strings.ToLower(level) == "debug",
		logger: logger,
	}

	return exporterhelper.NewMetricsExporter(
		config,
		logger,
		s.pushMetricsData,
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(logger)),
	)
}

// newLogsExporter creates an exporter.LogsExporter that just drops the
// received data and logs debugging messages.
func newLogsExporter(config configmodels.Exporter, level string, logger *zap.Logger) (component.LogsExporter, error) {
	s := &influxExporter{
		debug:  strings.ToLower(level) == "debug",
		logger: logger,
	}

	return exporterhelper.NewLogsExporter(
		config,
		logger,
		s.pushLogData,
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(logger)),
	)
}

func (s *influxExporter) pushLogData(
	_ context.Context,
	ld pdata.Logs,
) (int, error) {
	s.logger.Info("LogsExporter", zap.Int("#logs", ld.LogRecordCount()))

	if !s.debug {
		return 0, nil
	}

	buf := logDataBuffer{}
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		buf.logEntry("ResourceLog #%d", i)
		rl := rls.At(i)
		buf.logAttributeMap("Resource labels", rl.Resource().Attributes())
		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			buf.logEntry("InstrumentationLibraryLogs #%d", j)
			ils := ills.At(j)
			buf.logInstrumentationLibrary(ils.InstrumentationLibrary())

			logs := ils.Logs()
			for k := 0; k < logs.Len(); k++ {
				buf.logEntry("LogRecord #%d", k)
				lr := logs.At(k)
				buf.logLogRecord(lr)
			}
		}
	}

	s.logger.Debug(buf.str.String())

	return 0, nil
}

func loggerSync(logger *zap.Logger) func(context.Context) error {
	return func(context.Context) error {
		// Currently Sync() on stdout and stderr return errors on Linux and macOS,
		// respectively:
		//
		// - sync /dev/stdout: invalid argument
		// - sync /dev/stdout: inappropriate ioctl for device
		//
		// Since these are not actionable ignore them.
		err := logger.Sync()
		if osErr, ok := err.(*os.PathError); ok {
			wrappedErr := osErr.Unwrap()
			switch wrappedErr {
			case syscall.EINVAL, syscall.ENOTSUP, syscall.ENOTTY:
				err = nil
			}
		}
		return err
	}
}