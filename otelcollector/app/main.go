package main

import (
	"fmt"
	"net/http"
	"math/rand"
  "time"

  "github.com/prometheus/client_golang/prometheus"
  "github.com/prometheus/client_golang/prometheus/promauto"
  "github.com/prometheus/client_golang/prometheus/promhttp"
)

type TempInfo struct{
	minTemp int
	tempRange int
}

var (
	//seasons = []string{"spring", "summer", "winter", "fall"}
	//locations = []string{"midwest", "pnw", "south", "east"}
	/*locationsToMinTemp = map[string]map[string]int{
		"midwest": map[string]int{
			"spring": 40,
			"summer": 65,
			"winter": 0,
			"fall": 	30,
		},
		"pnw": map[string]int{
			"spring": 50,
			"summer": 60,
			"winter": 30,
			"fall": 	50,
		},
		"south": map[string]int{
			"spring": 65,
			"summer": 70,
			"winter": 50,
			"fall": 	60,
		},
		"east": map[string]int{
			"spring": 40,
			"summer": 65,
			"winter": 0,
			"fall": 	30,
		},
	}*/
	locationsToMinTemp = map[string]map[string]TempInfo {
		"midwest": map[string]TempInfo{
			"chicago": TempInfo{minTemp: 34, tempRange: 11},
			"minneapolis": TempInfo{minTemp: 24, tempRange: 20},
			"milwaukee": TempInfo{minTemp: 31, tempRange: 11},
			"indianapolis": TempInfo{minTemp: 31, tempRange: 19},
		},
		"pnw": map[string]TempInfo{
			"seattle": {42, 10},
			"portland": {41, 15},
			"tacoma": {37, 16},
			"bend": 	{27, 24},
		},
		"south": map[string]TempInfo{
			"atlanta": {42, 24},
			"orlando": {57, 22},
			"charleston": {51, 15},
		},
		"east": map[string]TempInfo{
			"new york": {36, 15},
			"boston": {31, 15},
			"dc": {35, 16},
			"baltimore": {39, 16},
		},
	}

	locationsToAvgRainfall = map[string]map[string]float64{
		"midwest": map[string]float64{
			"chicago": 0.07,
			"minneapolis": 0.02,
			"milwaukee": 0.01,
			"indianapolis": 0.063,
		},
		"pnw": map[string]float64{
			"seattle": 0.13,
			"portland": 0.15,
			"tacoma": 0.15,
			"bend": 	0.05,
		},
		"south": map[string]float64{
			"atlanta": 0.029,
			"orlando": 0.09,
			"charleston": 0.107,
		},
		"east": map[string]float64{
			"new york": 0.1,
			"boston": 0.113,
			"dc": 0.097,
			"baltimore": 0.074,
		},
	}
)

func recordMetrics() {
	go func() {
		i := 0
  	for {
			//seasonIndex := i % 4
			//season := seasons[seasonIndex]

			// new measurement
			//for _, location := range(locations) {
				//counter.WithLabelValues(season, location).Inc()
			//}

			for location, tempInfoByCity := range(locationsToMinTemp) {
				for city, info := range(tempInfoByCity) {
					counter.WithLabelValues(city, location).Inc()

					tempRange := info.tempRange
					minTemp := info.minTemp
					temperature := float64(rand.Intn(tempRange) + minTemp)
					gauge.WithLabelValues(city, location).Set(temperature)
					summary.WithLabelValues(city, location).Observe(temperature)
					histogram.WithLabelValues(city, location).Observe(temperature)
				}
			}

			for location, rainfallByCity := range(locationsToAvgRainfall) {
				for city, rainfall := range(rainfallByCity) {

					recordedRainfall := (float64(rand.Intn(10)) + rainfall * 100.0) / 100.0
					rainfallGauge.WithLabelValues(city, location).Set(recordedRainfall)
					rainfallSummary.WithLabelValues(city, location).Observe(recordedRainfall)
					rainfallHistogram.WithLabelValues(city, location).Observe(recordedRainfall)
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

var(
counter = promauto.NewCounterVec(
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
summary = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "myapp_temperature_summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	},
	[]string{
		"city",
		"location",
	},
)
rainfallSummary = promauto.NewSummaryVec(
	prometheus.SummaryOpts{
		Name: "myapp_rainfall_summary",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	},
	[]string{
		"city",
		"location",
	},
)
histogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "myapp_temperature_histogram",
		Buckets: prometheus.LinearBuckets(0, 10, 10),
	},
	[]string{
		"city",
		"location",
	},
)
rainfallHistogram = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "myapp_rainfall_histogram",
		Buckets: prometheus.LinearBuckets(0, 0.05, 10),
	},
	[]string{
		"city",
		"location",
	},
)
)

func main() {
  recordMetrics()

  http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
	
	fmt.Printf("ending main function")
}