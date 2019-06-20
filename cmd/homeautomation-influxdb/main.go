package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/orktes/homeautomation-metrics/internal/subscriber"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb "github.com/influxdata/influxdb-client-go"
)

func main() {
	mqttTopic := flag.String("mqtt-topic", "", "mqtt topic")
	mqttHostName := flag.String("mqtt-hostname", "localhost", "mqtt hostname")
	mqttPort := flag.Int("mqtt-port", 1883, "mqtt port")
	mqttClientID := flag.String("mqtt-client-id", "homeautomation-influxdb", "client id for MQTT")
	mqttUsername := flag.String("mqtt-username", "", "MQTT server username")
	mqttPassword := flag.String("mqtt-password", "", "MQTT server password")

	influxDBAddress := flag.String("influx-address", "localhost", "influxdb address")
	influxDBToken := flag.String("influx-token", "", "influxdb token")
	influxDBOrg := flag.String("influx-org", "", "influx organization name")
	influxDBBucket := flag.String("influx-bucket", "", "influx bucket name")

	flag.Parse()

	idMap := map[string]string{}

	for _, arg := range flag.Args() {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			continue
		}

		idMap[parts[0]] = idMap[parts[1]]
	}

	log.Printf("ID map%+v\n", idMap)

	influx, err := influxdb.New(http.DefaultClient, influxdb.WithAddress(*influxDBAddress), influxdb.WithToken(*influxDBToken))
	if err != nil {
		panic(err) // error handling here; normally we wouldn't use fmt but it works for the example
	}
	defer influx.Close()

	mqttOpts := getMQTTOptions(fmt.Sprintf("tcp://%s:%d", *mqttHostName, *mqttPort), *mqttClientID, *mqttUsername, *mqttPassword)
	mqttClient := mqtt.NewClient(mqttOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sub, err := subscriber.New(mqttClient, *mqttTopic, idMap)
	if err != nil {
		panic(err)
	}

	metricsLock := sync.Mutex{}
	metrics := []influxdb.Metric{}

	go func() {
		for metric := range sub.UpdateChan() {
			metricsLock.Lock()
			metrics = append(metrics, influxdb.NewRowMetric(
				metric.Fields,
				metric.Name,
				metric.Tags,
				metric.Time))
			log.Printf("%d metric entries in buffer", len(metrics))
			metricsLock.Unlock()
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		metricsLock.Lock()
		copyMetrics := make([]influxdb.Metric, len(metrics), len(metrics))
		copy(copyMetrics, metrics)
		metrics = metrics[:0]
		metricsLock.Unlock()

		if len(copyMetrics) == 0 {
			continue
		}

		log.Printf("writing %d metric entries to influxdb", len(copyMetrics))
		if err := influx.Write(context.Background(), *influxDBBucket, *influxDBOrg, copyMetrics...); err != nil {
			log.Printf("Influx write failed returning metrics to buffer %s", err.Error())
			metricsLock.Lock()
			if len(copyMetrics) > 100 {
				copyMetrics = copyMetrics[0:100]
			}
			metrics = append(metrics, copyMetrics...)
			metricsLock.Unlock()
		}
	}
}

func getMQTTOptions(server, clientID, username, password string) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts = opts.AddBroker(server)

	if clientID != "" {
		opts = opts.SetClientID(clientID)
	}
	if username != "" {
		opts = opts.SetUsername(username)
	}
	if password != "" {
		opts = opts.SetPassword(password)
	}

	return opts
}
