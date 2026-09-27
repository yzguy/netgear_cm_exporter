package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
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
	model    string
	baseUrl  string
	indexUrl string
	loginUrl string
	dataUrl  string
	username string
	password string

	mu sync.Mutex

	// Exporter metrics.
	totalScrapes prometheus.Counter
	scrapeErrors prometheus.Counter

	// Downstream QAM channel metrics.
	dsChannelPower                  *prometheus.Desc
	dsChannelSNR                    *prometheus.Desc
	dsChannelUnerroredCodewords     *prometheus.Desc
	dsChannelCorrectableCodewords   *prometheus.Desc
	dsChannelUncorrectableCodewords *prometheus.Desc

	// Upstream ATDMA channel metrics.
	usChannelPower *prometheus.Desc

	// Downstream OFDM channel metrics (DOCSIS 3.1, e.g. CM3000).
	dsOfdmChannelPower                  *prometheus.Desc
	dsOfdmChannelSNR                    *prometheus.Desc
	dsOfdmChannelUnerroredCodewords     *prometheus.Desc
	dsOfdmChannelCorrectableCodewords   *prometheus.Desc
	dsOfdmChannelUncorrectableCodewords *prometheus.Desc

	// Upstream OFDMA channel metrics (DOCSIS 3.1, e.g. CM3000).
	usOfdmaChannelPower *prometheus.Desc
}

// NewExporter returns an instance of Exporter configured with the modem's
// address, admin username, password and model.
func NewExporter(addr, username, password, model string) *Exporter {
	var (
		dsLabelNames      = []string{"channel", "lock_status", "modulation", "channel_id", "frequency"}
		usLabelNames      = []string{"channel", "lock_status", "modulation", "channel_id", "frequency"}
		dsOfdmLabelNames  = []string{"channel", "lock_status", "profile", "channel_id", "frequency"}
		usOfdmaLabelNames = []string{"channel", "lock_status", "profile", "channel_id", "frequency"}
	)

	e := &Exporter{
		// Modem access details.
		model:    model,
		baseUrl:  "http://" + addr,
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

		// Downstream QAM channel metrics.
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

		// Upstream ATDMA channel metrics.
		usChannelPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upstream_channel", "power_dbmv"),
			"Upstream channel power in dBmV.",
			usLabelNames, nil,
		),

		// Downstream OFDM channel metrics.
		dsOfdmChannelPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_ofdm_channel", "power_dbmv"),
			"Downstream OFDM channel power in dBmV.",
			dsOfdmLabelNames, nil,
		),
		dsOfdmChannelSNR: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_ofdm_channel", "snr_mer_db"),
			"Downstream OFDM channel signal to noise / modulation error ratio in dB.",
			dsOfdmLabelNames, nil,
		),
		dsOfdmChannelUnerroredCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_ofdm_channel", "unerrored_codewords_total"),
			"Downstream OFDM channel unerrored codewords.",
			dsOfdmLabelNames, nil,
		),
		dsOfdmChannelCorrectableCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_ofdm_channel", "correctable_codewords_total"),
			"Downstream OFDM channel correctable errors.",
			dsOfdmLabelNames, nil,
		),
		dsOfdmChannelUncorrectableCodewords: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "downstream_ofdm_channel", "uncorrectable_codewords_total"),
			"Downstream OFDM channel uncorrectable errors.",
			dsOfdmLabelNames, nil,
		),

		// Upstream OFDMA channel metrics.
		usOfdmaChannelPower: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upstream_ofdma_channel", "power_dbmv"),
			"Upstream OFDMA channel power in dBmV.",
			usOfdmaLabelNames, nil,
		),
	}

	switch model {
	case ModelCM3000:
		e.indexUrl = e.baseUrl + "/Login.htm"
		e.dataUrl = e.baseUrl + "/DocsisStatus.htm"
	default: // ModelCM1000
		e.indexUrl = e.baseUrl + "/GenieLogin.asp"
		e.loginUrl = e.baseUrl + "/goform/GenieLogin"
		e.dataUrl = e.baseUrl + "/DocsisStatus.asp"
	}

	return e
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
	// Downstream OFDM metrics.
	ch <- e.dsOfdmChannelPower
	ch <- e.dsOfdmChannelSNR
	ch <- e.dsOfdmChannelUnerroredCodewords
	ch <- e.dsOfdmChannelCorrectableCodewords
	ch <- e.dsOfdmChannelUncorrectableCodewords
	// Upstream OFDMA metrics.
	ch <- e.usOfdmaChannelPower
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

	e.mu.Lock()
	var err error
	switch e.model {
	case ModelCM3000:
		err = e.collectCM3000(ch)
	default: // ModelCM1000
		err = e.collectCM1000(ch)
	}
	if err != nil {
		log.Printf("scrape failed: %s", err)
		e.scrapeErrors.Inc()
	}
	e.totalScrapes.Collect(ch)
	e.scrapeErrors.Collect(ch)
	e.mu.Unlock()
}

// collectCM1000 scrapes a CM1000-style modem, where channel data is rendered
// server-side as static HTML tables.
func (e *Exporter) collectCM1000(ch chan<- prometheus.Metric) error {
	c := colly.NewCollector()

	// OnError callback logs any errors that occur during scraping. Registered
	// before the login POST below so it also covers that request.
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("scrape failed: %d %s", r.StatusCode, http.StatusText(r.StatusCode))
	})

	// Retrieve current webToken
	webToken, err := e.GetWebToken()
	if err != nil {
		return err
	}

	// Login to get a session cookie
	if err := c.Post(e.loginUrl, map[string]string{
		"loginUsername": e.username,
		"loginPassword": e.password,
		"login":         "1",
		"webToken":      webToken,
	}); err != nil {
		return err
	}

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
					freqMHz = hzTextToMHzLabel(text)
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
					freqMHz = hzTextToMHzLabel(text)
				case 5:
					fmt.Sscanf(text, "%f dBmV", &power)
				}
			})
			labels := []string{channel, lockStatus, modulation, channelID, freqMHz}

			ch <- prometheus.MustNewConstMetric(e.usChannelPower, prometheus.GaugeValue, power, labels...)
		})
	})

	return c.Visit(e.dataUrl)
}

// collectCM3000 scrapes a CM3000-style modem. Newer Netgear firmware no
// longer renders channel data as static HTML: DocsisStatus.htm instead
// embeds it as pipe-delimited strings inside inline <script> tags (see
// tagValueList in InitDsTableTagValue, InitUsTableTagValue, etc). Login also
// changed: Login.htm sets an XSRF cookie and its form's "action" attribute
// carries the login URL (including the required "id" query parameter).
func (e *Exporter) collectCM3000(ch chan<- prometheus.Metric) error {
	c := colly.NewCollector()

	var loginActionUrl string
	c.OnHTML(`form[name="loginform"]`, func(elem *colly.HTMLElement) {
		loginActionUrl = e.baseUrl + elem.Attr("action")
	})

	// Only capture <script> content from the data page itself: this
	// collector is reused for Login.htm too, and if the session isn't
	// authenticated DocsisStatus.htm may just redirect back to the login
	// page, so scoping this avoids silently parsing the wrong page's markup.
	var scriptText strings.Builder
	c.OnHTML(`script`, func(elem *colly.HTMLElement) {
		if elem.Request == nil || elem.Request.URL == nil || elem.Request.URL.String() != e.dataUrl {
			return
		}
		scriptText.WriteString(elem.Text)
		scriptText.WriteString("\n")
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("scrape failed: %d %s", r.StatusCode, http.StatusText(r.StatusCode))
	})

	// Visiting Login.htm captures the XSRF cookie (handled transparently by
	// colly's cookie jar) and the login form's action URL.
	if err := c.Visit(e.indexUrl); err != nil {
		return err
	}

	if loginActionUrl == "" {
		return fmt.Errorf("could not find login form action on %s", e.indexUrl)
	}

	if err := c.Post(loginActionUrl, map[string]string{
		"loginName":     e.username,
		"loginPassword": e.password,
	}); err != nil {
		return err
	}

	if err := c.Visit(e.dataUrl); err != nil {
		return err
	}

	script := scriptText.String()
	var parseErrs []error

	if dsChannels, err := parseDsQamChannels(script); err != nil {
		log.Printf("failed to parse downstream QAM channels: %s", err)
		parseErrs = append(parseErrs, err)
	} else {
		for _, ch2 := range dsChannels {
			// channelType (e.g. "ATDMA") fills the same "modulation" label
			// slot the CM1000 upstream table uses for its channel access
			// type, so the label set stays consistent across models.
			labels := []string{ch2.channel, ch2.lockStatus, ch2.modulation, ch2.channelID, ch2.frequency}
			ch <- prometheus.MustNewConstMetric(e.dsChannelPower, prometheus.GaugeValue, ch2.power, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelSNR, prometheus.GaugeValue, ch2.snr, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelCorrectableCodewords, prometheus.CounterValue, ch2.correctable, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsChannelUncorrectableCodewords, prometheus.CounterValue, ch2.uncorrectable, labels...)
		}
	}

	if usChannels, err := parseUsAtdmaChannels(script); err != nil {
		log.Printf("failed to parse upstream ATDMA channels: %s", err)
		parseErrs = append(parseErrs, err)
	} else {
		for _, ch2 := range usChannels {
			labels := []string{ch2.channel, ch2.lockStatus, ch2.channelType, ch2.channelID, ch2.frequency}
			ch <- prometheus.MustNewConstMetric(e.usChannelPower, prometheus.GaugeValue, ch2.power, labels...)
		}
	}

	if dsOfdmChannels, err := parseDsOfdmChannels(script); err != nil {
		log.Printf("failed to parse downstream OFDM channels: %s", err)
		parseErrs = append(parseErrs, err)
	} else {
		for _, ch2 := range dsOfdmChannels {
			labels := []string{ch2.channel, ch2.lockStatus, ch2.profile, ch2.channelID, ch2.frequency}
			ch <- prometheus.MustNewConstMetric(e.dsOfdmChannelPower, prometheus.GaugeValue, ch2.power, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsOfdmChannelSNR, prometheus.GaugeValue, ch2.snr, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsOfdmChannelUnerroredCodewords, prometheus.CounterValue, ch2.unerrored, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsOfdmChannelCorrectableCodewords, prometheus.CounterValue, ch2.correctable, labels...)
			ch <- prometheus.MustNewConstMetric(e.dsOfdmChannelUncorrectableCodewords, prometheus.CounterValue, ch2.uncorrectable, labels...)
		}
	}

	if usOfdmaChannels, err := parseUsOfdmaChannels(script); err != nil {
		log.Printf("failed to parse upstream OFDMA channels: %s", err)
		parseErrs = append(parseErrs, err)
	} else {
		for _, ch2 := range usOfdmaChannels {
			labels := []string{ch2.channel, ch2.lockStatus, ch2.profile, ch2.channelID, ch2.frequency}
			ch <- prometheus.MustNewConstMetric(e.usOfdmaChannelPower, prometheus.GaugeValue, ch2.power, labels...)
		}
	}

	// If every channel family failed to parse, something is structurally
	// wrong (e.g. an unauthenticated session redirected back to the login
	// page) rather than the modem simply lacking one channel type. Surface
	// that as a scrape error instead of reporting a quiet, empty success.
	if len(parseErrs) == 4 {
		return fmt.Errorf("failed to parse any channel data from %s: %w", e.dataUrl, parseErrs[0])
	}

	return nil
}

// hzTextToMHzLabel converts a "<n> Hz" string, as found in CM1000 HTML
// tables, into a "<n> MHz" label matching the exporter's existing format.
func hzTextToMHzLabel(text string) string {
	var freqHz float64
	fmt.Sscanf(text, "%f Hz", &freqHz)
	return fmt.Sprintf("%0.2f MHz", freqHz/1e6)
}

var (
	blockCommentRe  = regexp.MustCompile(`(?s)/\*.*?\*/`)
	nextFunctionRe  = regexp.MustCompile(`function\s+\w+\s*\(`)
	assignmentRe    = regexp.MustCompile(`(?s)tagValueList\s*=\s*([^;]+);`)
	stringLiteralRe = regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
)

// extractTagValueList finds the given JS function (e.g. InitDsTableTagValue)
// within script and returns the pipe-delimited fields assigned to its
// tagValueList variable, with any trailing empty field (from a trailing "|")
// removed.
//
// The function body is bounded by the start of the next top-level function
// (or end of script) rather than by brace-matching, since firmware in the
// wild has been observed building tagValueList by concatenating several
// quoted string literals (e.g. "8" + "|1|..." + "|2|...") rather than using
// a single literal, and any nested braces in a future firmware revision
// would otherwise truncate a naive single-brace match.
func extractTagValueList(script, funcName string) ([]string, error) {
	noComments := blockCommentRe.ReplaceAllString(script, "")

	startRe := regexp.MustCompile(`function\s+` + regexp.QuoteMeta(funcName) + `\s*\([^)]*\)`)
	loc := startRe.FindStringIndex(noComments)
	if loc == nil {
		return nil, fmt.Errorf("function %s not found", funcName)
	}

	body := noComments[loc[1]:]
	if next := nextFunctionRe.FindStringIndex(body); next != nil {
		body = body[:next[0]]
	}

	am := assignmentRe.FindStringSubmatch(body)
	if am == nil {
		return nil, fmt.Errorf("tagValueList not found in %s", funcName)
	}

	literals := stringLiteralRe.FindAllStringSubmatch(am[1], -1)
	if literals == nil {
		return nil, fmt.Errorf("tagValueList not found in %s", funcName)
	}

	var raw strings.Builder
	for _, lm := range literals {
		raw.WriteString(lm[1])
		raw.WriteString(lm[2])
	}

	fields := strings.Split(raw.String(), "|")
	if len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	return fields, nil
}

// channelGroups splits fields (a leading channel count followed by
// fixed-size groups of per-channel values) into those groups.
func channelGroups(fields []string, groupSize int) ([][]string, error) {
	if len(fields) < 1 {
		return nil, fmt.Errorf("expected a leading channel count, got no fields")
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil {
		return nil, fmt.Errorf("invalid channel count %q: %w", fields[0], err)
	}

	rest := fields[1:]
	if len(rest) < count*groupSize {
		return nil, fmt.Errorf("expected %d fields for %d channels, got %d", count*groupSize, count, len(rest))
	}

	groups := make([][]string, 0, count)
	for i := 0; i < count; i++ {
		groups = append(groups, rest[i*groupSize:(i+1)*groupSize])
	}
	return groups, nil
}

type dsQamChannel struct {
	channel, lockStatus, modulation, channelID, frequency string
	power, snr, correctable, uncorrectable                float64
}

// parseDsQamChannels parses the InitDsTableTagValue tagValueList: Channel |
// Lock Status | Modulation | Channel ID | Frequency | Power | SNR |
// Correctables | Uncorrectables.
func parseDsQamChannels(script string) ([]dsQamChannel, error) {
	fields, err := extractTagValueList(script, "InitDsTableTagValue")
	if err != nil {
		return nil, err
	}
	groups, err := channelGroups(fields, 9)
	if err != nil {
		return nil, err
	}

	channels := make([]dsQamChannel, 0, len(groups))
	for _, g := range groups {
		c := dsQamChannel{
			channel:    g[0],
			lockStatus: g[1],
			modulation: g[2],
			channelID:  g[3],
			frequency:  hzTextToMHzLabel(g[4]),
		}
		fmt.Sscanf(g[5], "%f", &c.power)
		fmt.Sscanf(g[6], "%f", &c.snr)
		fmt.Sscanf(g[7], "%f", &c.correctable)
		fmt.Sscanf(g[8], "%f", &c.uncorrectable)
		channels = append(channels, c)
	}
	return channels, nil
}

type usAtdmaChannel struct {
	channel, lockStatus, channelType, channelID, frequency string
	power                                                  float64
}

// parseUsAtdmaChannels parses the InitUsTableTagValue tagValueList: Channel |
// Lock Status | US Channel Type | Channel ID | Symbol Rate | Frequency |
// Power. The symbol rate is dropped to keep the same label set as the
// CM1000's upstream channel metric.
func parseUsAtdmaChannels(script string) ([]usAtdmaChannel, error) {
	fields, err := extractTagValueList(script, "InitUsTableTagValue")
	if err != nil {
		return nil, err
	}
	groups, err := channelGroups(fields, 7)
	if err != nil {
		return nil, err
	}

	channels := make([]usAtdmaChannel, 0, len(groups))
	for _, g := range groups {
		c := usAtdmaChannel{
			channel:     g[0],
			lockStatus:  g[1],
			channelType: g[2],
			channelID:   g[3],
			frequency:   hzTextToMHzLabel(g[5]),
		}
		fmt.Sscanf(g[6], "%f dBmV", &c.power)
		channels = append(channels, c)
	}
	return channels, nil
}

type dsOfdmChannel struct {
	channel, lockStatus, profile, channelID, frequency string
	power, snr                                         float64
	unerrored, correctable, uncorrectable              float64
}

// parseDsOfdmChannels parses the InitDsOfdmTableTagValue tagValueList:
// Channel | Lock Status | Profile | Channel ID | Frequency | Power |
// SNR/MER | Active Subcarrier Range | Unerrored Codewords | Correctable
// Codewords | Uncorrectable Codewords.
func parseDsOfdmChannels(script string) ([]dsOfdmChannel, error) {
	fields, err := extractTagValueList(script, "InitDsOfdmTableTagValue")
	if err != nil {
		return nil, err
	}
	groups, err := channelGroups(fields, 11)
	if err != nil {
		return nil, err
	}

	channels := make([]dsOfdmChannel, 0, len(groups))
	for _, g := range groups {
		c := dsOfdmChannel{
			channel:    g[0],
			lockStatus: g[1],
			profile:    strings.TrimSpace(g[2]),
			channelID:  g[3],
			frequency:  hzTextToMHzLabel(g[4]),
		}
		fmt.Sscanf(g[5], "%f dBmV", &c.power)
		fmt.Sscanf(g[6], "%f dB", &c.snr)
		fmt.Sscanf(g[8], "%f", &c.unerrored)
		fmt.Sscanf(g[9], "%f", &c.correctable)
		fmt.Sscanf(g[10], "%f", &c.uncorrectable)
		channels = append(channels, c)
	}
	return channels, nil
}

type usOfdmaChannel struct {
	channel, lockStatus, profile, channelID, frequency string
	power                                              float64
}

// parseUsOfdmaChannels parses the InitUsOfdmaTableTagValue tagValueList:
// Channel | Lock Status | Profile | Channel ID | Frequency | Power.
func parseUsOfdmaChannels(script string) ([]usOfdmaChannel, error) {
	fields, err := extractTagValueList(script, "InitUsOfdmaTableTagValue")
	if err != nil {
		return nil, err
	}
	groups, err := channelGroups(fields, 6)
	if err != nil {
		return nil, err
	}

	channels := make([]usOfdmaChannel, 0, len(groups))
	for _, g := range groups {
		c := usOfdmaChannel{
			channel:    g[0],
			lockStatus: g[1],
			profile:    strings.TrimSpace(g[2]),
			channelID:  g[3],
			frequency:  hzTextToMHzLabel(g[4]),
		}
		fmt.Sscanf(g[5], "%f dBmV", &c.power)
		channels = append(channels, c)
	}
	return channels, nil
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

	exporter := NewExporter(config.Modem.Address, config.Modem.Username, config.Modem.Password, config.Modem.Model)

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
