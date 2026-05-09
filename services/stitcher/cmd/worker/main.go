package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("🚀 Booting up Archon Stitcher Worker...")

	// TODO: Initialize OpenTelemetry, Kafka Consumer, and Docker client here.

	// Listen for OS signals to initiate a graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down Stitcher Worker gracefully...")
}