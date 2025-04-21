package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
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
	interval := 5 * time.Second // значение по умолчанию

	if val := os.Getenv("BME_UPDATE_INTERVAL"); val != "" {
		if sec, err := strconv.Atoi(val); err == nil && sec > 0 {
			interval = time.Duration(sec) * time.Second
			log.Println("Set update interval to", interval)
		} else {
			log.Println("Invalid BME_UPDATE_INTERVAL, using default:", interval)
		}
	}

	for {
		d, err := i2c.Open(&i2c.Devfs{Dev: "/dev/" + device}, addr)
		if err != nil {
			log.Println("i2c open error:", err)
			time.Sleep(interval)
			continue
		}

		b := bme280.New(d)
		if err := b.Init(); err != nil {
			log.Println("bme280 init error:", err)
			d.Close()
			time.Sleep(interval)
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

		time.Sleep(interval)
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
