package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "netgear_cm"

var (
	version   string
	revision  string
	branch    string
	buildUser string
	buildDate string
)

// Exporter represents an instance of the Netgear cable modem exporter.
type Exporter struct {
	indexUrl string
	loginUrl string
	dataUrl  string
	username string
	password string

	mu sync.Mutex

	// Exporter metrics.
	totalScrapes prometheus.Counter
	scrapeErrors prometheus.Counter

	// Downstream metrics.
	dsChannelPower                  *prometheus.Desc
	dsChannelSNR                    *prometheus.Desc
	dsChannelUnerroredCodewords     *prometheus.Desc
	dsChannelCorrectableCodewords   *prometheus.Desc
	dsChannelUncorrectableCodewords *prometheus.Desc

	// Upstream metrics.
	usChannelPower *prometheus.Desc
}

// NewExporter returns an instance of Exporter configured with the modem's
// address, admin username and password.
func NewExporter(addr, username, password string) *Exporter {
	var (
		dsLabelNames = []string{"channel", "lock_status", "modulation", "channel_id", "frequency"}
		usLabelNames = []string{"channel", "lock_status", "modulation", "channel_id", "frequency"}
	)

	return &Exporter{
		// Modem access details.
		indexUrl: "http://" + addr + "/GenieLogin.asp",
		loginUrl: "http://" + addr + "/goform/GenieLogin",
		dataUrl:  "http://" + addr + "/DocsisStatus.asp",
		username: username,
		password: password,

		// Collection metrics.
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "status_scrapes_total",
			Help:      "Total number of scrapes of the modem status page.",
		}),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "status_scrape_errors_total",
			Help:      "Total number of failed scrapes of the modem status page.",
		}),

		// Downstream metrics.
		dsChannelPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_channel", "power_dbmv"),
			"Downstream channel power in dBmV.",
			dsLabelNames, nil,
		),
		dsChannelSNR: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_channel", "snr_db"),
			"Downstream channel signal to noise ratio in dB.",
			dsLabelNames, nil,
		),
		dsChannelUnerroredCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_channel", "unerrored_codewords_total"),
			"Downstream channel correctable errors.",
			dsLabelNames, nil,
		),
		dsChannelCorrectableCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_channel", "correctable_codewords_total"),
			"Downstream channel correctable errors.",
			dsLabelNames, nil,
		),
		dsChannelUncorrectableCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_channel", "uncorrectable_codewords_total"),
			"Downstream channel uncorrectable errors.",
			dsLabelNames, nil,
		),

		// Upstream metrics.
		usChannelPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upstream_channel", "power_dbmv"),
			"Upstream channel power in dBmV.",
			usLabelNames, nil,
		),
	}
}

// Describe returns Prometheus metric descriptions for the exporter metrics.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// Exporter metrics.
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeErrors.Desc()
	// Downstream metrics.
	ch <- e.dsChannelPower
	ch <- e.dsChannelSNR
	ch <- e.dsChannelUnerroredCodewords
	ch <- e.dsChannelCorrectableCodewords
	ch <- e.dsChannelUncorrectableCodewords
	// Upstream metrics.
	ch <- e.usChannelPower
}

func (e *Exporter) GetWebToken() (string, error) {
	resp, err := http.Get(e.indexUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	webToken, _ := doc.Find(`input[name="webToken"]`).Attr("value")

	return webToken, nil
}

// Collect runs our scrape loop returning each Prometheus metric.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.totalScrapes.Inc()

	c := colly.NewCollector()

	// Retrieve current webToken
	webToken, err := e.GetWebToken()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Current web token: %s", webToken)

	// Login to get a session cookie
	err = c.Post(e.loginUrl, map[string]string{
		"loginUsername": e.username,
		"loginPassword": e.password,
		"login":         "1",
		"webToken":      webToken,
	})
	if err != nil {
		log.Fatal(err)
	}

	// OnError callback counts any errors that occur during scraping.
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("scrape failed: %d %s", r.StatusCode, http.StatusText(r.StatusCode))
		e.scrapeErrors.Inc()
	})

	// Callback to parse the tbody block of table with id=dsTable, the downstream table info.
	c.OnHTML(`#dsTable tbody`, func(elem *colly.HTMLElement) {
		elem.DOM.Find("tr").Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return // no rows were returned
			}
			var (
				channel                string
				lockStatus             string
				modulation             string
				channelID              string
				freqMHz                string
				power                  float64
				snr                    float64
				unerroredCodewords     float64
				correctableCodewords   float64
				uncorrectableCodewords float64
			)
			row.Find("td").Each(func(j int, col *goquery.Selection) {
				text := strings.TrimSpace(col.Text())

				switch j {
				case 0:
					channel = text
				case 1:
					lockStatus = text
				case 2:
					modulation = text
				case 3:
					channelID = text
				case 4:
					{
						var freqHZ float64
						fmt.Sscanf(text, "%f Hz", &freqHZ)
						freqMHz = fmt.Sprintf("%0.2f MHz", freqHZ/1e6)
					}
				case 5:
					fmt.Sscanf(text, "%f dBmV", &power)
				case 6:
					fmt.Sscanf(text, "%f dB", &snr)
				case 7:
					fmt.Sscanf(text, "%f", &unerroredCodewords)
				case 8:
					fmt.Sscanf(text, "%f", &correctableCodewords)
				case 9:
					fmt.Sscanf(text, "%f", &uncorrectableCodewords)
				}
			})
			labels := []string{channel, lockStatus, modulation, channelID, freqMHz}

			ch <- prometheus.MustNewConstMetric(e.dsChannelPower, prometheus.GaugeValue, power, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelSNR, prometheus.GaugeValue, snr, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelUnerroredCodewords, prometheus.CounterValue, unerroredCodewords, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelCorrectableCodewords, prometheus.CounterValue, correctableCodewords, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelUncorrectableCodewords, prometheus.CounterValue, uncorrectableCodewords, labels...)
		})
	})

	// Callback to parse the tbody block of table with id=usTable, the upstream channel info.
	c.OnHTML(`#usTable tbody`, func(elem *colly.HTMLElement) {
		elem.DOM.Find("tr").Each(func(i int, row *goquery.Selection) {
			if i == 0 {
				return // no rows were returned
			}
			var (
				channel    string
				lockStatus string
				modulation string
				channelID  string
				freqMHz    string
				power      float64
			)
			row.Find("td").Each(func(j int, col *goquery.Selection) {
				text := strings.TrimSpace(col.Text())
				switch j {
				case 0:
					channel = text
				case 1:
					lockStatus = text
				case 2:
					modulation = text
				case 3:
					channelID = text
				case 4:
					{
						var freqHZ float64
						fmt.Sscanf(text, "%f Hz", &freqHZ)
						freqMHz = fmt.Sprintf("%0.2f MHz", freqHZ/1e6)
					}
				case 5:
					fmt.Sscanf(text, "%f dBmV", &power)
				}
			})
			labels := []string{channel, lockStatus, modulation, channelID, freqMHz}

			ch <- prometheus.MustNewConstMetric(e.usChannelPower, prometheus.GaugeValue, power, labels...)
		})
	})

	e.mu.Lock()
	c.Visit(e.dataUrl)
	e.totalScrapes.Collect(ch)
	e.scrapeErrors.Collect(ch)
	e.mu.Unlock()
}

func main() {
	var (
		configFile  = flag.String("config.file", "netgear_cm_exporter.yml", "Path to configuration file.")
		showVersion = flag.Bool("version", false, "Print version information.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("netgear_cm_exporter version=%s revision=%s branch=%s buildUser=%s buildDate=%s\n",
			version, revision, branch, buildUser, buildDate)
		os.Exit(0)
	}

	config, err := NewConfigFromFile(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	exporter := NewExporter(config.Modem.Address, config.Modem.Username, config.Modem.Password)

	prometheus.MustRegister(exporter)

	http.Handle(config.Telemetry.MetricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, config.Telemetry.MetricsPath, http.StatusMovedPermanently)
	})

	log.Printf("exporter listening on %s", config.Telemetry.ListenAddress)
	if err := http.ListenAndServe(config.Telemetry.ListenAddress, nil); err != nil {
		log.Fatalf("failed to start netgear exporter: %s", err)
	}
}
