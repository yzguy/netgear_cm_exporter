package main

import (
	"testing"
)

func TestNewExporterURLsByModel(t *testing.T) {
	cases := []struct {
		model                          string
		wantIndex, wantLogin, wantData string
	}{
		{
			model:     ModelCM1000,
			wantIndex: "http://192.168.100.1/GenieLogin.asp",
			wantLogin: "http://192.168.100.1/goform/GenieLogin",
			wantData:  "http://192.168.100.1/DocsisStatus.asp",
		},
		{
			model:     ModelCM3000,
			wantIndex: "http://192.168.100.1/Login.htm",
			wantLogin: "",
			wantData:  "http://192.168.100.1/DocsisStatus.htm",
		},
	}

	for _, c := range cases {
		e := NewExporter("192.168.100.1", "admin", "secret", c.model)
		if e.indexUrl != c.wantIndex {
			t.Errorf("model %s: indexUrl = %q, want %q", c.model, e.indexUrl, c.wantIndex)
		}
		if e.loginUrl != c.wantLogin {
			t.Errorf("model %s: loginUrl = %q, want %q", c.model, e.loginUrl, c.wantLogin)
		}
		if e.dataUrl != c.wantData {
			t.Errorf("model %s: dataUrl = %q, want %q", c.model, e.dataUrl, c.wantData)
		}
	}
}

// concatenatedScript exercises the historical string-concatenation form for
// tagValueList (seen in commented-out examples in real firmware, e.g. "8" +
// "|1|..." + "|2|..."), which a prior version of extractTagValueList could
// not parse.
const concatenatedScript = `
function InitUsTableTagValue()
{
    var tagValueList = "4" +
        "|1|Not Locked|Unknown|0|0|0|0.0" +
        "|2|Not Locked|Unknown|0|0|0|0.0" +
        "|3|Not Locked|Unknown|0|0|0|0.0" +
        "|4|Not Locked|Unknown|0|0|0|0.0";

    return tagValueList.split("|");
}
`

func TestExtractTagValueListConcatenatedLiterals(t *testing.T) {
	fields, err := extractTagValueList(concatenatedScript, "InitUsTableTagValue")
	if err != nil {
		t.Fatalf("extractTagValueList: %v", err)
	}
	want := []string{
		"4",
		"1", "Not Locked", "Unknown", "0", "0", "0", "0.0",
		"2", "Not Locked", "Unknown", "0", "0", "0", "0.0",
		"3", "Not Locked", "Unknown", "0", "0", "0", "0.0",
		"4", "Not Locked", "Unknown", "0", "0", "0", "0.0",
	}
	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(fields), len(want), fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Errorf("fields[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}

// nestedBraceScript exercises a function body containing a brace pair before
// the tagValueList assignment, which a naive non-greedy single-brace match
// would truncate at.
const nestedBraceScript = `
function InitDsTableTagValue()
{
    if (needsRefresh) { doRefresh(); }
    var tagValueList = '1|1|Locked|QAM256|1|100000000 Hz|1.0|40|1|0';

    return tagValueList.split("|");
}
`

func TestExtractTagValueListNestedBraces(t *testing.T) {
	fields, err := extractTagValueList(nestedBraceScript, "InitDsTableTagValue")
	if err != nil {
		t.Fatalf("extractTagValueList: %v", err)
	}
	want := []string{"1", "1", "Locked", "QAM256", "1", "100000000 Hz", "1.0", "40", "1", "0"}
	if len(fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %v", len(fields), len(want), fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Errorf("fields[%d] = %q, want %q", i, fields[i], want[i])
		}
	}
}

// cm3000Script is the literal DocsisStatus.htm script content reported in
// https://github.com/yzguy/netgear_cm_exporter/issues/1, used to validate
// the tagValueList extraction and parsing against real device output.
const cm3000Script = `
function InitTagValue()
{
/*
  Acquire Downstream Channel (text) | Acquire Downstream Channel Comment (text) |
  Connectivity State (text) | Connectivity State Comment (text) |
  Boot State (text) | Boot State Comment (text) |
  Configuration File (text) | Configuration File Comment (text) |
  Security (text) | Security Comment (text) |
  Current System Time (text)
*/
    var tagValueList = '489000000|Locked|OK|Operational|OK|Operational|&nbsp;|&nbsp;|Enabled|BPI+|Tue Oct 21 05:56:04 2025|0|0|0|105 days 14:17:17|3|1|';

    return tagValueList.split("|");
}

function InitUsTableTagValue()
{
/*
  Channel (text) | Lock Status (text) | US Channel Type (text) | Channel ID (text) | Symbol Rate (text) | Frequency (text) | Power (text)
*/
/*
    var tagValueList = "4" +
        "|1|Not Locked|Unknown|0|0|0|0.0" +
        "|2|Not Locked|Unknown|0|0|0|0.0" +
        "|3|Not Locked|Unknown|0|0|0|0.0" +
        "|4|Not Locked|Unknown|0|0|0|0.0";
*/
    var tagValueList = '8|1|Locked|ATDMA|4|5120 Ksym/sec|35600000 Hz|41.3 dBmV|2|Locked|ATDMA|2|5120 Ksym/sec|22800000 Hz|40.8 dBmV|3|Locked|ATDMA|3|5120 Ksym/sec|29200000 Hz|41.0 dBmV|4|Locked|ATDMA|1|5120 Ksym/sec|16400000 Hz|40.8 dBmV|5|Not Locked|Unknown|0|0|0|0.0|6|Not Locked|Unknown|0|0|0|0.0|7|Not Locked|Unknown|0|0|0|0.0|8|Not Locked|Unknown|0|0|0|0.0|';

    return tagValueList.split("|");
}

function InitDsTableTagValue()
{
/*
  Channel (text) | Lock Status (text) | Modulation (text) | Channel ID (text) | Frequency (text) | Power (text) | SNR (text) | Correctables (text) | Uncorrectables (text)
*/
/*
    var tagValueList = "8" +
        "|1|Locked|Unknown|0|809500000|-61.6|0.0|11|0" +
        "|2|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|3|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|4|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|5|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|6|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|7|Not Locked|Unknown|0|0|0.0|0.0|0|0" +
        "|8|Not Locked|Unknown|0|0|0.0|0.0|0|0";
*/
    var tagValueList = '32|1|Locked|QAM256|20|489000000 Hz|1.9|41.8|7099|638|2|Locked|QAM256|13|447000000 Hz|1.9|42|346|931|3|Locked|QAM256|14|453000000 Hz|1.7|42|305|942|4|Locked|QAM256|15|459000000 Hz|1.8|42|306|972|5|Locked|QAM256|16|465000000 Hz|1.8|41.9|387|960|6|Locked|QAM256|17|471000000 Hz|1.9|41.9|373|937|7|Locked|QAM256|18|477000000 Hz|2|42|344|959|8|Locked|QAM256|19|483000000 Hz|1.8|41.7|291|956|9|Locked|QAM256|21|495000000 Hz|1.8|41.6|280|946|10|Locked|QAM256|22|501000000 Hz|1.9|41.6|307|941|11|Locked|QAM256|23|507000000 Hz|1.6|41.8|313|974|12|Locked|QAM256|24|513000000 Hz|1.6|41.8|318|969|13|Locked|QAM256|25|519000000 Hz|1.7|42|346|946|14|Locked|QAM256|26|525000000 Hz|1.6|41.8|315|955|15|Locked|QAM256|27|531000000 Hz|1.7|41.9|286|975|16|Locked|QAM256|28|537000000 Hz|1.5|41.9|3493|516|17|Locked|QAM256|29|543000000 Hz|1.6|41.8|309|917|18|Locked|QAM256|30|549000000 Hz|1.3|41.7|291|942|19|Locked|QAM256|31|555000000 Hz|1.2|42|327|983|20|Locked|QAM256|32|561000000 Hz|1.5|41.8|311|981|21|Locked|QAM256|33|567000000 Hz|1.5|42|389|968|22|Locked|QAM256|34|573000000 Hz|1.5|42.3|375|975|23|Locked|QAM256|35|579000000 Hz|1.3|42|429|986|24|Locked|QAM256|36|585000000 Hz|1.3|41.8|330|972|25|Locked|QAM256|37|591000000 Hz|1.2|41.8|287|932|26|Locked|QAM256|38|597000000 Hz|1.2|42.3|311|919|27|Locked|QAM256|39|603000000 Hz|1.4|42|327|941|28|Locked|QAM256|40|609000000 Hz|1.3|42|320|992|29|Locked|QAM256|41|615000000 Hz|1.2|41.9|302|985|30|Locked|QAM256|42|621000000 Hz|1.3|42|313|974|31|Locked|QAM256|43|627000000 Hz|1.3|42|260|1009|32|Locked|QAM256|44|633000000 Hz|1.5|41.7|311|992|';

    return tagValueList.split("|");
}

function InitUsOfdmaTableTagValue()
{
    /*
    var tagValueList = '2'
        + '|1||Success|1300000 Hz|74~1673|18|30.8 dBmV'
        + '|2||Success|41300000 Hz|74~1673|18|30.5 dBmV';
    */
    var tagValueList = '2|1|Locked|12 ,13|41|36200000 Hz|36.3 dBmV|2|Not Locked|0|0|0 Hz|0 dBmV';

    return tagValueList.split("|");
}

function InitDsOfdmTableTagValue()
{
    /*
    var tagValueList = '2'
        + '|1|66|Primary|297600000 Hz|148~3947|0|0'
        + '|2|99|Backup Primary|495600000 Hz|148~3947|0|0';
    */
    var tagValueList = '2|1|Locked|0 ,1 ,2 ,3|193|690000000 Hz|2.78 dBmV|42.2 dB|328 ~ 3767|167630374573|162793792928|721|2|Locked|0 ,1 ,2 ,3|194|957000000 Hz|3.48 dBmV|41.6 dB|148 ~ 3947|169139688110|118533652152|30991|';

    return tagValueList.split("|");
}
`

func TestParseDsQamChannels(t *testing.T) {
	channels, err := parseDsQamChannels(cm3000Script)
	if err != nil {
		t.Fatalf("parseDsQamChannels: %v", err)
	}
	if len(channels) != 32 {
		t.Fatalf("got %d channels, want 32", len(channels))
	}

	first := channels[0]
	want := dsQamChannel{
		channel:       "1",
		lockStatus:    "Locked",
		modulation:    "QAM256",
		channelID:     "20",
		frequency:     "489.00 MHz",
		power:         1.9,
		snr:           41.8,
		correctable:   7099,
		uncorrectable: 638,
	}
	if first != want {
		t.Errorf("first channel = %+v, want %+v", first, want)
	}

	last := channels[31]
	if last.channel != "32" || last.channelID != "44" || last.frequency != "633.00 MHz" || last.uncorrectable != 992 {
		t.Errorf("last channel = %+v", last)
	}
}

func TestParseUsAtdmaChannels(t *testing.T) {
	channels, err := parseUsAtdmaChannels(cm3000Script)
	if err != nil {
		t.Fatalf("parseUsAtdmaChannels: %v", err)
	}
	if len(channels) != 8 {
		t.Fatalf("got %d channels, want 8", len(channels))
	}

	first := channels[0]
	want := usAtdmaChannel{
		channel:     "1",
		lockStatus:  "Locked",
		channelType: "ATDMA",
		channelID:   "4",
		frequency:   "35.60 MHz",
		power:       41.3,
	}
	if first != want {
		t.Errorf("first channel = %+v, want %+v", first, want)
	}

	notLocked := channels[4]
	if notLocked.lockStatus != "Not Locked" || notLocked.channel != "5" {
		t.Errorf("5th channel = %+v", notLocked)
	}
}

func TestParseDsOfdmChannels(t *testing.T) {
	channels, err := parseDsOfdmChannels(cm3000Script)
	if err != nil {
		t.Fatalf("parseDsOfdmChannels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("got %d channels, want 2", len(channels))
	}

	first := channels[0]
	want := dsOfdmChannel{
		channel:       "1",
		lockStatus:    "Locked",
		profile:       "0 ,1 ,2 ,3",
		channelID:     "193",
		frequency:     "690.00 MHz",
		power:         2.78,
		snr:           42.2,
		unerrored:     167630374573,
		correctable:   162793792928,
		uncorrectable: 721,
	}
	if first != want {
		t.Errorf("first channel = %+v, want %+v", first, want)
	}

	second := channels[1]
	if second.channelID != "194" || second.frequency != "957.00 MHz" || second.uncorrectable != 30991 {
		t.Errorf("second channel = %+v", second)
	}
}

func TestParseUsOfdmaChannels(t *testing.T) {
	channels, err := parseUsOfdmaChannels(cm3000Script)
	if err != nil {
		t.Fatalf("parseUsOfdmaChannels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("got %d channels, want 2", len(channels))
	}

	first := channels[0]
	want := usOfdmaChannel{
		channel:    "1",
		lockStatus: "Locked",
		profile:    "12 ,13",
		channelID:  "41",
		frequency:  "36.20 MHz",
		power:      36.3,
	}
	if first != want {
		t.Errorf("first channel = %+v, want %+v", first, want)
	}

	second := channels[1]
	if second.lockStatus != "Not Locked" || second.power != 0 {
		t.Errorf("second channel = %+v", second)
	}
}
