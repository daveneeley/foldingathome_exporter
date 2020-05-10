package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakotoE/go-fahapi"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace         = "foldingathome"
	subsystemSlot     = "slot"
	subsystemWorkUnit = "work_unit"
)

type Exporter struct {
	address string
	logger  log.Logger

	up                                 *prometheus.Desc
	uptime                             *prometheus.Desc
	time                               *prometheus.Desc
	version                            *prometheus.Desc
	slotStatus                         *prometheus.Desc
	slotAttempts                       *prometheus.Desc
	slotNextAttempt                    *prometheus.Desc
	slotEstimatedPointsPerDay          *prometheus.Desc
	workUnitStepsCompletedPercent      *prometheus.Desc
	workUnitCreditEstimatePoints       *prometheus.Desc
	workUnitEstimatedCompletionSeconds *prometheus.Desc
	workUnitTimeRemainingSeconds       *prometheus.Desc
}

func NewExporter(address string, logger log.Logger) *Exporter {
	return &Exporter{
		address: address,
		logger:  logger,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the FAHClient be reached.",
			nil,
			nil,
		),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "uptime_seconds"),
			"Number of seconds since the FAHClient started.",
			nil,
			nil,
		),
		time: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "time_seconds"),
			"Current UNIX time according to the server.",
			nil,
			nil,
		),
		version: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "version"),
			"The version of this FAHClient.",
			[]string{"version"},
			nil,
		),
		slotStatus: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemSlot, "status"),
			"The status of the slot, encoded numerically. 0 => uknown, 1 => ready, 2 => download, 3 => running, 4 => upload, 5 => finishing, 6 => stopping, 7 => paused",
			[]string{"id", "slot_description"},
			nil,
		),
		slotAttempts: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemSlot, "attempts"),
			"Number of attempts to download a work unit.",
			[]string{"id", "slot_description"},
			nil,
		),
		slotNextAttempt: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemSlot, "next_attempt_seconds"),
			"Seconds until the next attempt to download a work unit.",
			[]string{"id", "slot_description"},
			nil,
		),
		slotEstimatedPointsPerDay: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemSlot, "estimated_points_per_day"),
			"Estimated number of points the slot can produce in a day.",
			[]string{"id", "slot_description"},
			nil,
		),
		workUnitStepsCompletedPercent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemWorkUnit, "steps_completed_percent"),
			"Work unit completion percentage.",
			[]string{"id", "slot_description", "prcg"},
			nil,
		),
		workUnitCreditEstimatePoints: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemWorkUnit, "credit_estimate_points"),
			"Estimated number of points that will be credited for the work unit.",
			[]string{"id", "slot_description", "prcg"},
			nil,
		),
		workUnitEstimatedCompletionSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemWorkUnit, "estimated_completion_seconds"),
			"Estimated seconds until the work unit is completed.",
			[]string{"id", "slot_description", "prcg"},
			nil,
		),
		workUnitTimeRemainingSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemWorkUnit, "time_remaining_seconds"),
			"Seconds until the work unit's deadline, after which the work unit is expired and will be discarded by the client.",
			[]string{"id", "slot_description", "prcg"},
			nil,
		),
	}
}

// Describe describes all the metrics exported by the foldingathome exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	ch <- e.uptime
	ch <- e.time
	ch <- e.version
	ch <- e.slotStatus
	ch <- e.slotAttempts
	ch <- e.slotNextAttempt
	ch <- e.workUnitStepsCompletedPercent
	ch <- e.workUnitCreditEstimatePoints
	ch <- e.workUnitEstimatedCompletionSeconds
	ch <- e.workUnitTimeRemainingSeconds
}

// Collect fetches the statistics from the configured foldingathome server, and
// delivers them as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	api, err := fahapi.NewAPI(e.address)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
		level.Error(e.logger).Log("msg", "Failed to connect to FAHClient", "err", err)
		return
	}
	defer api.Close()

	up := float64(1)
	uptime, err := api.Exec("eval \"$(uptime)\\n\"")
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect uptime from FAHClient", "err", err)
		up = 0
	}
	date, err := api.Exec("eval \"$(date)\\n\"")
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect date from FAHClient", "err", err)
		up = 0
	}
	info, err := api.Info()
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect info from FAHClient", "err", err)
		up = 0
	}
	slotInfo, err := api.SlotInfo()
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect slot-info from FAHClient", "err", err)
		up = 0
	}
	queueInfo, err := api.QueueInfo()
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to collect queue-info from FAHClient", "err", err)
		up = 0
	}

	if err := e.parseUptime(ch, uptime); err != nil {
		up = 0
	}
	if err := e.parseDate(ch, date); err != nil {
		up = 0
	}
	if err := e.parseInfo(ch, info); err != nil {
		up = 0
	}
	if err := e.parseSlotInfo(ch, slotInfo); err != nil {
		up = 0
	}
	if err := e.parseQueueInfo(ch, slotInfo, queueInfo); err != nil {
		up = 0
	}

	ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, up)
}

// example uptime: "14h 31m  2s\"
func (e *Exporter) parseUptime(ch chan<- prometheus.Metric, uptime string) error {
	// TODO: ParseDuration doesn't handle days, but fahapi.parseFAHDuration isn't exported
	d, err := time.ParseDuration(strings.ReplaceAll(strings.TrimSuffix(uptime, "\\"), " ", ""))
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to parse uptime", "err", err)
		return err
	}

	ch <- prometheus.MustNewConstMetric(e.uptime, prometheus.GaugeValue, d.Seconds())

	return err
}

// example date: "2020-05-09T20:04:48Z\"
func (e *Exporter) parseDate(ch chan<- prometheus.Metric, date string) error {
	t, err := time.Parse(time.RFC3339, strings.TrimSuffix(date, "\\"))
	if err != nil {
		level.Error(e.logger).Log("msg", "Failed to parse date", "err", err)
		return err
	}

	ch <- prometheus.MustNewConstMetric(e.time, prometheus.GaugeValue, float64(t.Unix()))

	return err
}

// example info:
// [
//   [
//     "FAHClient",
//     ["Version", "7.6.9"],
//     ["...", "...]
//   ],
//   [ ... ]
// ]
func (e *Exporter) parseInfo(ch chan<- prometheus.Metric, info [][]interface{}) error {
	for _, section := range info {
		if section[0].(string) == "FAHClient" {
			for _, pairs := range section[1:] {
				typedPairs := pairs.([]interface{})
				if typedPairs[0].(string) == "Version" {
					ch <- prometheus.MustNewConstMetric(e.version, prometheus.GaugeValue, 1, typedPairs[1].(string))
					return nil
				}
			}
		}
	}

	err := errors.New("Version not found in info response")
	level.Error(e.logger).Log("msg", "Failed to parse version", "err", err)

	return err
}

func (e *Exporter) parseSlotInfo(ch chan<- prometheus.Metric, slotInfo []fahapi.SlotInfo) error {
	statusMap := map[string]float64{
		"ready":     1,
		"download":  2,
		"running":   3,
		"upload":    4,
		"finishing": 5,
		"stopping":  6,
		"paused":    7,
	}

	for _, info := range slotInfo {
		ch <- prometheus.MustNewConstMetric(e.slotStatus, prometheus.GaugeValue, statusMap[strings.ToLower(info.Status)], info.ID, info.Description)
	}

	return nil
}

func (e *Exporter) parseQueueInfo(ch chan<- prometheus.Metric, slotInfo []fahapi.SlotInfo, queueInfo []fahapi.SlotQueueInfo) error {
	slotMap := map[string]fahapi.SlotInfo{}
	for _, sInfo := range slotInfo {
		slotMap[sInfo.ID] = sInfo
	}

	for _, qInfo := range queueInfo {
		id := slotMap[qInfo.Slot].ID
		desc := slotMap[qInfo.Slot].Description
		prcg := fmt.Sprintf("%d (%d, %d, %d)", qInfo.Project, qInfo.Run, qInfo.Clone, qInfo.Gen)
		state := strings.ToLower(qInfo.State)

		if state == "download" {
			ch <- prometheus.MustNewConstMetric(e.slotAttempts, prometheus.GaugeValue, float64(qInfo.Attempts), id, desc)
			ch <- prometheus.MustNewConstMetric(e.slotNextAttempt, prometheus.GaugeValue, qInfo.NextAttempt.Seconds(), id, desc)
		}

		if state == "running" || state == "finishing" {
			ch <- prometheus.MustNewConstMetric(e.slotEstimatedPointsPerDay, prometheus.GaugeValue, float64(qInfo.PPD), id, desc)
		}

		if !(qInfo.Project == 0 && qInfo.Run == 0 && qInfo.Clone == 0 && qInfo.Gen == 0) {
			percentDone, err := strconv.ParseFloat(strings.TrimSuffix(qInfo.PercentDone, "%"), 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(e.workUnitStepsCompletedPercent, prometheus.GaugeValue, percentDone, id, desc, prcg)
			}

			ch <- prometheus.MustNewConstMetric(e.workUnitCreditEstimatePoints, prometheus.GaugeValue, float64(qInfo.CreditEstimate), id, desc, prcg)
			ch <- prometheus.MustNewConstMetric(e.workUnitEstimatedCompletionSeconds, prometheus.GaugeValue, qInfo.ETA.Seconds(), id, desc, prcg)
			ch <- prometheus.MustNewConstMetric(e.workUnitTimeRemainingSeconds, prometheus.GaugeValue, qInfo.TimeRemaining.Seconds(), id, desc, prcg)
		}
	}

	return nil
}

func main() {
	var (
		address       = kingpin.Flag("fahclient.address", "Folding@home client telnet API address.").Default("localhost:36330").String()
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9737").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
	)
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting foldingathome_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	prometheus.MustRegister(NewExporter(*address, logger))

	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Folding@home Exporter</title></head>
             <body>
             <h1>Folding@home Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error running HTTP server", "err", err)
		os.Exit(1)
	}
}
