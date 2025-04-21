package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/quhar/bme280"
	"golang.org/x/exp/io/i2c"
)

type SensorData struct {
	T, P, H float64
	mu      sync.RWMutex
}

var sensor SensorData

func main() {
	const (
		device  = "i2c-1" // имя файла в /dev
		address = 0x76    // I2C адрес сенсора
	)

	// запуск фонового обновления данных
	go updateLoop(device, address)

	http.HandleFunc("/metrics", metricsHandler)

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func updateLoop(device string, addr int) {
	for {
		d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/" + device}, addr)
		if err != nil {
			log.Println("i2c open error:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		b := bme280.New(d)
		if err := b.Init(); err != nil {
			log.Println("bme280 init error:", err)
			d.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		t, p, h, err := b.EnvData()
		d.Close()
		if err != nil {
			log.Println("env read error:", err)
		} else {
			sensor.mu.Lock()
			sensor.T, sensor.P, sensor.H = t, p, h
			sensor.mu.Unlock()
		}

		time.Sleep(5 * time.Second)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	sensor.mu.RLock()
	defer sensor.mu.RUnlock()

	fmt.Fprintf(w, "# HELP bme280_temperature_celsius Temperature in Celsius\n")
	fmt.Fprintf(w, "# TYPE bme280_temperature_celsius gauge\n")
	fmt.Fprintf(w, "bme280_temperature_celsius %f\n", sensor.T)

	fmt.Fprintf(w, "# HELP bme280_pressure_hpa Pressure in hPa\n")
	fmt.Fprintf(w, "# TYPE bme280_pressure_hpa gauge\n")
	fmt.Fprintf(w, "bme280_pressure_hpa %f\n", sensor.P)

	fmt.Fprintf(w, "# HELP bme280_humidity_percent Relative humidity in %%\n")
	fmt.Fprintf(w, "# TYPE bme280_humidity_percent gauge\n")
	fmt.Fprintf(w, "bme280_humidity_percent %f\n", sensor.H)
}
