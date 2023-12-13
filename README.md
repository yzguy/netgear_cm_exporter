# Netgear Cable Modem Exporter

Prometheus exporter for NETGEAR cable modems

## Supported Devices

These Netgear models have been tested and are officially supported:

* Netgear CM1000

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