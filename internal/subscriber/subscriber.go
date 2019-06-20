package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/orktes/homeautomation-metrics/internal/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MetricChanBuffer length of the buffered channel for metrics
const MetricChanBuffer = 50

// Subscriber subscribes to a MQTT topic and emits metrics
type Subscriber struct {
	c     mqtt.Client
	topic string
	idMap map[string]string

	data       map[string]types.Metric
	updateChan chan types.Metric
	closeChan  chan struct{}

	sync.Mutex
}

// New returns a new Subscriber for a given mqtt client and topic prefix
func New(c mqtt.Client, topic string, idMap map[string]string) (*Subscriber, error) {
	s := &Subscriber{
		c:          c,
		topic:      topic,
		idMap:      idMap,
		data:       map[string]types.Metric{},
		updateChan: make(chan types.Metric, MetricChanBuffer),
		closeChan:  make(chan struct{}),
	}

	return s, s.init()
}

// UpdateChan returns a channel for Metric updates
func (s *Subscriber) UpdateChan() <-chan types.Metric {
	return s.updateChan
}

// CloseChan returns a channel that is closed when subscriber close
func (s *Subscriber) CloseChan() <-chan struct{} {
	return s.closeChan
}

// Close closes the subscriber
func (s *Subscriber) Close(ctx context.Context) error {
	if token := s.c.Unsubscribe(s.topic); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	close(s.closeChan)
	close(s.updateChan)
	return nil
}

func (s *Subscriber) onMessage(client mqtt.Client, msg mqtt.Message) {
	s.Lock()
	defer s.Unlock()

	metricName, tags, field := s.parseTopic(msg.Topic())
	if metricName == "" {
		return
	}

	cacheKey := fmt.Sprintf("%s:%s", metricName, tags["id"])
	metric, ok := s.data[cacheKey]
	if !ok {
		metric = types.Metric{
			Name:   metricName,
			Fields: map[string]interface{}{},
			Tags:   map[string]string{},
		}
	}

	metric.Tags = tags

	value, err := s.parseValue(metricName, msg.Payload())
	if err != nil {
		// TODO print error
		return
	}

	if value != nil {
		metric.Fields[field] = value
	}
	metric.Time = time.Now()

	s.data[cacheKey] = metric

	s.sendMetric(metric.Clone())
}

func (s *Subscriber) sendMetric(m types.Metric) {
	t := time.NewTimer(time.Second * 5)
	defer t.Stop()

	select {
	case <-s.closeChan:
		return
	default:
		select {
		case s.updateChan <- m:
			return
		case <-t.C:
			// TODO log dropped messages
			log.Printf("dropped message %+v, chan len %d\n", m, len(s.updateChan))
			return
		}
	}
}

func (s *Subscriber) parseValue(metricName string, payload []byte) (value interface{}, err error) {
	if len(payload) == 0 {
		return
	}

	firstChar := payload[0]
	if firstChar != '{' && firstChar != '[' {
		var res interface{}
		err = json.Unmarshal(payload, &res)
		if err != nil {
			return
		}

		return res, nil
	}

	var val payloadValue

	err = json.Unmarshal(payload, &val)
	if err != nil {
		return
	}

	return s.parseValue(metricName, val.Value)
}

func (s *Subscriber) parseTopic(topic string) (metricName string, tags map[string]string, field string) {
	parts := strings.Split(topic, "/")
	if len(parts) < 5 {
		// TODO print error
		return
	}

	id := parts[len(parts)-2]

	metricName = strings.Join(parts[0:len(parts)-2], "_")
	tags = map[string]string{
		"id": id,
	}
	mappedID, ok := s.idMap[fmt.Sprintf("%s:%s", metricName, id)]
	if ok {
		tags["name"] = mappedID
	}

	field = parts[len(parts)-1]

	return
}

func (s *Subscriber) init() error {
	if token := s.c.Subscribe(s.topic, 1, s.onMessage); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	if token := s.c.Publish(fmt.Sprintf("%s/get", strings.Split(s.topic, "/")[0]), 1, false, []byte{}); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}
