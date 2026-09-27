# Netgear Cable Modem Exporter

Prometheus exporter for NETGEAR cable modems

## Supported Devices

These Netgear models have been tested and are officially supported:

* Netgear CM1000
* Netgear CM3000

Set `modem.model` in your config file to `CM1000` (default) or `CM3000` to match your device. Newer
Netgear firmware (as used on the CM3000) exposes docsis status through a different login flow and
embeds channel data as JavaScript rather than static HTML, so the two models are scraped differently
under the hood but expose the same core downstream/upstream QAM and ATDMA channel metrics. One
exception: `netgear_cm_downstream_channel_unerrored_codewords_total` is CM1000-only, since the
CM3000's DOCSIS status page doesn't expose an unerrored codeword count for those channels. The
CM3000 additionally exposes metrics (including unerrored codewords) for its DOCSIS 3.1 OFDM/OFDMA
channels, which the CM1000 doesn't have.

## Installation

You can build and install the exporter locally by running:

```
go get github.com/yzguy/netgear_cm_exporter
```

## Usage

```
Usage of ./netgear_cm_exporter:
  -config.file string
    	Path to configuration file. (default "netgear_cm_exporter.yml")
  -version
    	Print version information.
```

An example configuration file is provided in `netgear_cm_exporter.yml` showing all the possible
configuration options. The values in the example are the defaults

You can also specify password via an environment variable

* `NETGEAR_MODEM_PASSWORD`

```
modem:
  password: <your password here>
```

## Grafana Dashboard

A sample grafana dashboard can be found in the `grafana/` directory. You can import `netgear_cable_modem.json` into 
your Grafana instance to get up and running with a quick dashboard.

![Grafana Dashboard Screenshot](/grafana/dashboard_screenshot.png)