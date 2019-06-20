package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/armon/go-metrics/prometheus"
	"github.com/orktes/homeautomation-metrics/internal/collector"
	"github.com/orktes/homeautomation-metrics/internal/subscriber"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	mqttTopic := flag.String("mqtt-topic", "", "mqtt topic")
	mqttHostName := flag.String("mqtt-hostname", "localhost", "mqtt hostname")
	mqttPort := flag.Int("mqtt-port", 1883, "mqtt port")
	mqttClientID := flag.String("mqtt-client-id", "homeautomation-influxdb", "client id for MQTT")
	mqttUsername := flag.String("mqtt-username", "", "MQTT server username")
	mqttPassword := flag.String("mqtt-password", "", "MQTT server password")
	httpPort := flag.String("http-port", ":8080", "Port for prometheus metrics route")

	flag.Parse()

	mqttOpts := getMQTTOptions(fmt.Sprintf("tcp://%s:%d", *mqttHostName, *mqttPort), *mqttClientID, *mqttUsername, *mqttPassword)
	mqttClient := mqtt.NewClient(mqttOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sub, err := subscriber.New(mqttClient, *mqttTopic, map[string]string{})
	if err != nil {
		panic(err)
	}

	prometheusSink, err := prometheus.NewPrometheusSinkFrom(prometheus.PrometheusOpts{Expiration: time.Duration(0)})
	if err != nil {
		panic(err)
	}

	col := collector.New(sub.UpdateChan(), prometheusSink)
	defer col.Close()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*httpPort, nil))
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
