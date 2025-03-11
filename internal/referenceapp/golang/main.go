package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type TempInfo struct {
	minTemp   int
	tempRange int
}

var (
	locationsToMinTempPerf = map[string]map[string]TempInfo{
		"midwest": map[string]TempInfo{
			"chicago":      TempInfo{minTemp: 34, tempRange: 11},
			"minneapolis":  TempInfo{minTemp: 24, tempRange: 20},
			"milwaukee":    TempInfo{minTemp: 31, tempRange: 11},
			"indianapolis": TempInfo{minTemp: 31, tempRange: 19},
		},
		"pnw": map[string]TempInfo{
			"seattle":  {42, 10},
			"portland": {41, 15},
			"tacoma":   {37, 16},
			"bend":     {27, 24},
		},
	}
	locationsToMinTemp = map[string]map[string]TempInfo{
		"midwest": map[string]TempInfo{
			"chicago":      TempInfo{minTemp: 34, tempRange: 11},
			"minneapolis":  TempInfo{minTemp: 24, tempRange: 20},
			"milwaukee":    TempInfo{minTemp: 31, tempRange: 11},
			"indianapolis": TempInfo{minTemp: 31, tempRange: 19},
		},
		"pnw": map[string]TempInfo{
			"seattle":  {42, 10},
			"portland": {41, 15},
			"tacoma":   {37, 16},
			"bend":     {27, 24},
		},
		"south": map[string]TempInfo{
			"atlanta":    {42, 24},
			"orlando":    {57, 22},
			"charleston": {51, 15},
		},
		"east": map[string]TempInfo{
			"new york":  {36, 15},
			"boston":    {31, 15},
			"dc":        {35, 16},
			"baltimore": {39, 16},
		},
	}

	locationsToAvgRainfall = map[string]map[string]float64{
		"midwest": map[string]float64{
			"chicago":      0.07,
			"minneapolis":  0.02,
			"milwaukee":    0.01,
			"indianapolis": 0.063,
		},
		"pnw": map[string]float64{
			"seattle":  0.13,
			"portland": 0.15,
			"tacoma":   0.15,
			"bend":     0.05,
		},
		"south": map[string]float64{
			"atlanta":    0.029,
			"orlando":    0.09,
			"charleston": 0.107,
		},
		"east": map[string]float64{
			"new york":  0.1,
			"boston":    0.113,
			"dc":        0.097,
			"baltimore": 0.074,
		},
	}

	locationsToAvgRainfallMaxDimensions = map[string]map[string]float64{
		"midwest": map[string]float64{
			"chicago":       0.07,
			"minneapolis":   0.02,
			"milwaukee":     0.01,
			"indianapolis":  0.063,
			"seattle":       0.13,
			"portland":      0.15,
			"tacoma":        0.15,
			"bend":          0.05,
			"atlanta":       0.029,
			"orlando":       0.09,
			"charleston":    0.107,
			"new york":      0.1,
			"boston":        0.113,
			"dc":            0.097,
			"baltimore":     0.074,
			"Mumbai":        0.07,
			"Delhi":         0.02,
			"Bangalore":     0.01,
			"Hyderabad":     0.063,
			"Ahmedabad":     0.13,
			"Chennai":       0.15,
			"Kolkata":       0.15,
			"Surat":         0.05,
			"Pune":          0.029,
			"Jaipur":        0.09,
			"Lucknow":       0.107,
			"Kanpur":        0.1,
			"Nagpur":        0.113,
			"Indore":        0.097,
			"Bhopal":        0.074,
			"Chicago":       0.07,
			"Minneapolis":   0.02,
			"Milwaukee":     0.01,
			"Indianapolis":  0.063,
			"Seattle":       0.13,
			"Portland":      0.15,
			"Tacoma":        0.15,
			"Bend":          0.05,
			"Atlanta":       0.029,
			"Orlando":       0.09,
			"Charleston":    0.107,
			"New york":      0.1,
			"Boston":        0.113,
			"Dc":            0.097,
			"Baltimore":     0.074,
			"mumbai":        0.07,
			"delhi":         0.02,
			"bangalore":     0.01,
			"hyderabad":     0.063,
			"ahmedabad":     0.13,
			"chennai":       0.15,
			"kolkata":       0.15,
			"surat":         0.05,
			"pune":          0.029,
			"jaipur":        0.09,
			"lucknow":       0.107,
			"kanpur":        0.1,
			"nagpur":        0.113,
			"indore":        0.097,
			"bhopal":        0.074,
			"Patna":         0.113,
			"Visakhapatnam": 0.097,
			"Ranchi":        0.074,
		},
		"pnw": map[string]float64{
			"seattle":  0.13,
			"portland": 0.15,
			"tacoma":   0.15,
			"bend":     0.05,
		},
		"south": map[string]float64{
			"atlanta":    0.029,
			"orlando":    0.09,
			"charleston": 0.107,
		},
		"east": map[string]float64{
			"new york":  0.1,
			"boston":    0.113,
			"dc":        0.097,
			"baltimore": 0.074,
		},
	}

	locationsWithEmptyDimensions = map[string]map[string]float64{
		"": map[string]float64{
			"": 0.07,
		},
	}

	locationsWithUpperLimitMetricLength = map[string]map[string]float64{
		"upperLocationjJVQNohonMTtBTjTzUCDQoTtcvKWQKGBrVPeDrjqnOUhGHtEGSwaPBgcwYRZHkWYUWkKQJOTBNUcUwGJuvHPNTNgunuuTxrtEPpVBzXfFfXzbVtQZUoWWMYhenryHWThwrrpcAOjjddjncPtZFAnxcuAodryUutosqMXPpPHEkAbOMjmPSZcRbahrrWYbcyGPBwDmwXrZCqjsYtxxnDVFDDHffseYdZASfHcwWJhrAFVVdxyPswcd": map[string]float64{
			"upperCityeFqyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYdphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNpEVHczWymZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywwXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftszHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcervn": 0.989012567389034562368280504644755455752678715981192959635310676073764844661892261815462840418898492978406130512210667448759338405320703922453754330588152792124564796123612353040012940416629878863176287823052621161587118380534508926549156339527783955800407023012439838534880556891527842361290159549237934605580899318038707983071369492067211626076531201182788959889669289178294720967479755811786324133464974314918779963890926904768906857611391542779705153213815449163555280809421330235536372742444851556984308096132471194363558694451677387428763681425730797875003107509427427054931069773806939192613918018961813161091185062970086550791313488065662937468254349147612248848804277583685342025000972539111587251139459190275399388824961745858140041052024887800235595496244299543776793052927561470368747738859300765201991829249727486953152303358828431090591826197835098586330116508600843471777783321025865973622229418353364161340344405647389221020650345377134239902165043673155133734119270325369516419856042979217304223904389573146968552,
		},
	}
)

func recordMetrics() {
	go func() {
		i := 0
		for {
			for location, tempInfoByCity := range locationsToMinTemp {
				for city, info := range tempInfoByCity {
					counter.WithLabelValues(city, location).Inc()

					tempRange := info.tempRange
					minTemp := info.minTemp
					temperature := float64(rand.Intn(tempRange) + minTemp)
					gauge.WithLabelValues(city, location).Set(temperature)
					summary.WithLabelValues(city, location).Observe(temperature)
					histogram.WithLabelValues(city, location).Observe(temperature)
				}
			}

			for location, rainfallByCity := range locationsToAvgRainfall {
				for city, rainfall := range rainfallByCity {

					recordedRainfall := (float64(rand.Intn(10)) + rainfall*100.0) / 100.0
					rainfallGauge.WithLabelValues(city, location).Set(recordedRainfall)
					rainfallSummary.WithLabelValues(city, location).Observe(recordedRainfall)
					rainfallHistogram.WithLabelValues(city, location).Observe(recordedRainfall)
				}
			}

			for location, rainfallByCity := range locationsWithEmptyDimensions {
				for city, rainfall := range rainfallByCity {

					emptyDimensionRainfall := (float64(rand.Intn(10)) + rainfall*100.0) / 100.0
					emptyRainfallGauge.WithLabelValues(city, location).Set(emptyDimensionRainfall)
					emptyRainfallSummary.WithLabelValues(city, location).Observe(emptyDimensionRainfall)
					emptyDimensionHistogram.WithLabelValues(city, location).Observe(emptyDimensionRainfall)
				}
			}

			for location, rainfallByCity := range locationsToAvgRainfallMaxDimensions {
				for city, rainfall := range rainfallByCity {

					maxDimensionRainfall := (float64(rand.Intn(10)) + rainfall*100.0) / 100.0
					maxDimensionRainfallGauge.WithLabelValues(city, location).Set(maxDimensionRainfall)
					maxDimensionRainfallSummary.WithLabelValues(city, location).Observe(maxDimensionRainfall)
					maxDimensionRainfallHistogram.WithLabelValues(city, location).Observe(maxDimensionRainfall)
				}
			}

			for location, rainfallByCity := range locationsWithUpperLimitMetricLength {
				for city, rainfall := range rainfallByCity {

					upperLimitDimensionRainfall := (float64(rand.Intn(10)) + rainfall*100.0) / 100.0
					upperLimitRainfallGauge.WithLabelValues(city, location).Set(upperLimitDimensionRainfall)
					upperLimitRainfallSummary.WithLabelValues(city, location).Observe(upperLimitDimensionRainfall)
					upperLimitRainfallDimensionHistogram.WithLabelValues(city, location).Observe(upperLimitDimensionRainfall)
				}
			}

			i++
			// Wait the scrape interval
			for j := 0; j < 60; j++ {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func recordPerfMetrics() {
	go func() {
		i := 0
		for {
			for _, gauge := range gaugeList {
				for location, tempInfoByCity := range locationsToMinTempPerf {
					for city, info := range tempInfoByCity {
						tempRange := info.tempRange
						minTemp := info.minTemp
						temperature := float64(rand.Intn(tempRange) + minTemp)
						gauge.WithLabelValues(city, location).Set(temperature)
					}
				}

				i++
			}
			// Wait the scrape interval
			for j := 0; j < scrapeIntervalSec; j++ {
				time.Sleep(1 * time.Second)
			}
		}
	}()
}

func createGauges() {
	for i := 0; i < metricCount; i++ {
		name := fmt.Sprintf("myapp_temperature_%d", i)
		gauge := promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
			},
			[]string{
				"city",
				"location",
			},
		)
		gaugeList = append(gaugeList, gauge)
	}
}

var (
	scrapeIntervalSec = 60
	metricCount       = 10000
	gaugeList         = make([]*prometheus.GaugeVec, 0, metricCount)
	counter           = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "myapp_measurements_total",
		},
		[]string{
			"city",
			"location",
		},
	)
	gauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myapp_temperature",
		},
		[]string{
			"city",
			"location",
		},
	)
	rainfallGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "myapp_rainfall",
		},
		[]string{
			"city",
			"location",
		},
	)

	emptyRainfallGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "empty_dimension_rainfall",
		},
		[]string{
			"city",
			"location",
		},
	)
	maxDimensionRainfallGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "max_dimension_rainfall",
		},
		[]string{
			"city",
			"location",
		},
	)
	upperLimitRainfallGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "upperGaugeFqyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYephoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWymZEGRx_UbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftgzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer",
		},
		[]string{
			"city",
			"location",
		},
	)
	summary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "myapp_temperature_summary",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{
			"city",
			"location",
		},
	)
	rainfallSummary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "myapp_rainfall_summary",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{
			"city",
			"location",
		},
	)
	upperLimitRainfallSummary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "upperSummaryyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNrEVHc_WymZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwffthzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{
			"city",
			"location",
		},
	)
	maxDimensionRainfallSummary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "max_dimension_rainfall_summary",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{
			"city",
			"location",
		},
	)
	emptyRainfallSummary = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "empty_dimension_summary",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{
			"city",
			"location",
		},
	)
	histogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "myapp_temperature_histogram",
			Buckets: prometheus.LinearBuckets(0, 10, 10),
		},
		[]string{
			"city",
			"location",
		},
	)
	rainfallHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "myapp_rainfall_histogram",
			Buckets: prometheus.LinearBuckets(0, 0.05, 10),
		},
		[]string{
			"city",
			"location",
		},
	)
	upperLimitRainfallDimensionHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "upperHistogramtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWy_ZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywtrXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftkzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer",
			Buckets: prometheus.LinearBuckets(0, 0.05, 10),
		},
		[]string{
			"city",
			"location",
		},
	)
	maxDimensionRainfallHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "max_dimension_rainfall_histogram",
			Buckets: prometheus.LinearBuckets(0, 0.05, 10),
		},
		[]string{
			"city",
			"location",
		},
	)
	emptyDimensionHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "empty_dimension_histogram",
			Buckets: prometheus.LinearBuckets(0, 0.05, 10),
		},
		[]string{
			"city",
			"location",
		},
	)
)

func untypedHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "untyped_metric{label_0=\"label-value\"} 0")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "untyped_metric{label_1=\"label-value\"} 1")
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()
	if !ok {
		fmt.Println("Error parsing basic auth")
		w.WriteHeader(401)
		fmt.Fprintf(w, "basic auth error")
		return
	}
	if u != "rashmi" {
		fmt.Printf("Username provided is incorrect: %s\n", u)
		w.WriteHeader(401)
		fmt.Fprintf(w, "username error")
		return
	}
	if p != "pwd" {
		fmt.Printf("Password provided is incorrect: %s\n", u)
		w.WriteHeader(401)
		fmt.Fprintf(w, "pwd error")
		return
	}
	fmt.Printf("Username: %s\n", u)
	fmt.Printf("Password: %s\n", p)
	w.WriteHeader(200)
	fmt.Fprintf(w, "my_metric{label_0=\"label-value\"} 0")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "my_metric{label_1=\"label-value\"} 1")
	return
}

func main() {

	// certFile := "/etc/prometheus/certs/client-cert.pem"
	// keyFile := "/etc/prometheus/certs/client-key.pem"
	if os.Getenv("RUN_PERF_TEST") == "true" {
		if os.Getenv("SCRAPE_INTERVAL") != "" {
			scrapeIntervalSec, _ = strconv.Atoi(os.Getenv("SCRAPE_INTERVAL"))
		}
		if os.Getenv("METRIC_COUNT") != "" {
			metricCount, _ = strconv.Atoi(os.Getenv("METRIC_COUNT"))
		}
		createGauges()
		recordPerfMetrics()
	} else {
		recordMetrics()
	}

	untypedServer := http.NewServeMux()
	untypedServer.HandleFunc("/metrics", untypedHandler)
	weatherServer := http.NewServeMux()
	weatherServer.Handle("/metrics", promhttp.Handler())

	handler := http.HandlerFunc(handleRequest)
	http.Handle("/example", handler)
	http.ListenAndServe(":2114", nil)

	// Run server for metrics without a type
	go func() {
		http.ListenAndServe(":2113", untypedServer)
	}()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("HTTP server failed to start: %v", r)
		}
	}()

	// Run main server for weather app metrics
	// err := http.ListenAndServeTLS(":2112", certFile, keyFile, weatherServer)
	// if err != nil {
	// 	log.Printf("HTTP server failed to start: %v", err)
	// }

	fmt.Printf("ending main function")
}
