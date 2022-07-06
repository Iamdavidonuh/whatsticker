package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	rmq "github.com/adjust/rmq/v4"
	"github.com/deven96/whatsticker/metrics"
)

func listenForCtrlC() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func main() {

	gauges := metrics.NewGauges()
	registry := metrics.NewRegistry()
	metric := metrics.Initialize(registry, gauges)

	log.Printf("Initialized Metrics Consumer %#v", metric)

	errChan := make(chan error)
	connectionString := os.Getenv("WAIT_HOSTS")
	connection, err := rmq.OpenConnection("logger connection", "tcp", connectionString, 1, errChan)
	if err != nil {
		log.Print("Failed to connect to redis queue")
		return
	}
	loggingQueue, _ := connection.OpenQueue(os.Getenv("LOG_METRIC_QUEUE"))
	loggingQueue.StartConsuming(10, time.Second)
	loggingQueue.AddConsumer("logging-consumer", &metric)
	log.Printf("Starting Queue on %s", connectionString)
	listenForCtrlC()
}
