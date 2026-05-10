package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	BuildsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stitcher_builds_total",
			Help: "The total number of processed build requests",
		},
		[]string{"status"}, // "success" or "error"
	)

	BuildDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "stitcher_build_duration_seconds",
			Help:    "Histogram of build processing times",
			Buckets: prometheus.DefBuckets,
		},
	)

	DockerErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "stitcher_docker_errors_total",
			Help: "The total number of docker execution failures",
		},
	)
)