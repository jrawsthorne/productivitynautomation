# Grafana Dashboard Automation

This python service provides an API for programatically creating grafana dashboards. It also acts as a proxy between grafana and your data to allow grafana to use a JSON or CSV file or a Couchbase bucket as a data source.

# Requirements

- Python 3
- Pip
- Grafana
- Grafana JSON plugin

# Setup Grafana

Install grafana https://grafana.com/docs/grafana/latest/installation/ and setup a username and password

Install the JSON plugin https://grafana.com/grafana/plugins/simpod-json-datasource/installation

# Setup all services

1) Setup API, grafana, prometheus, 
 cd docker
 docker-compose up

2) Create a new dashboard
 Take the input metadata json (modify the data sources and visual elements) and import into the Dashboard API service. 

 Example: 
    curl -X POST -H “Content-Type: application/json” -d @./templates/server_dynamic_vms.json http://localhost:5000/import

  This gives the <Dashboard ID>

3) Visit  the dashboard
  http://localhost:grafana_port/<dashboardid>

If any changes needed in the dashboard resources/visual, then rerun the above import into API service curl command.


# Run the service

The only command line argument is the connection string for grafana in the format of username:password@host:port

```
pip install -r requirements.txt
python dashboard.py http://admin:password@127.0.0.1:3000
```

The API is now listening on port 5000

# Docker deployment

The easiest way to get prometheus, grafana, exporter.py and dashboard.py working is with docker

A Dockerfile is provided for dashboard.py and exporter.py and the docker folder contains the necessary files for a full deployment

(TODO: Deduplicate the staging and production folders)

From within the docker/staging folder, running docker-compose up will start grafana on port 4000 and dashboard.py on port 5001 as well as prometheus and the exporter

Persistent data will be stored in:

config/grafana for grafana
config/queries.json for exporter.py
config/targets.json for dashboad.py
config/prometheus.yml for prometheus configuration (TODO: persistent prometheus storage for database)

# Backup

Use the Grafana Backup Tool [https://github.com/ysde/grafana-backup-tool](https://github.com/ysde/grafana-backup-tool) to backup grafana dashboards and datasources - follow the instructions in the README
Everything else is stored in this repository directly. 

# Restore

Use the Grafana Backup Tool [https://github.com/ysde/grafana-backup-tool](https://github.com/ysde/grafana-backup-tool) and point it at the backup file stored in docker/*/config/*.tar.gz
Everything else is stored in this repository directly. 

# Using the API

There are 2 relevant endpoints /add and /import.

/add is used to add data sources and Grafana dashboards with a simpler custom json format.

/import is also used to import data source and grafana dashboards however it uses the grafana JSON format for the UI section allowing more flexibility.

/export/:uid is used to export a grafana dashboard as well as the data sources that dashboard uses. The result can be used directly in /import to move dashboards around easily

For any request you don't understand it may be useful to view the next section which contains a comprehensive example

A request for /export/:uid is a GET request where uid is replaced with the unique ID of the dashboard you wish to export. This id can be found in the url when viewing a dashboard e.g. http:// your-grafana-hostname/d/:uid/your-dashboard-name

A request for /add should be in this format:

```json
{
  "data": [],
  "grafana": [],
  "dashboard_title": ""
}
```

A request for /import should be in this format:

```json
{
  "data": [],
  "grafana": {},
  // if a dashboad exists with the same name, it is only overwritten if overwrite is true otherwise an error is returned
  "overwrite": "<true|false>"
}
```

Each item in the data array should be formatted as follows:

```json
{
  "source": "couchbase|json|csv",
  "type": "timeseries|table",
  "name": "<unique name for this data source>",
  "refresh": "<how often the data should be refreshed (seconds)>"
}
```

Extra fields are required depending on the source and type of data

Couchbase source

```json
{
  "host": "<couchbase host>",
  "username": "<couchbase username>",
  "password": "<couchbase password>",
  "query": "<couchbase query>"
}
```

JSON or CSV source

```json
{
  "file": "<link to file served via http>"
}
```

JSON source

```json
{
  // single value is true if the json file is a single value rather than an array of values
  "single_value": "<true|false>"
}
```

CSV source

```json
{
  // how the values in the csv are separated
  "delimiter": "<comma, space, tab or custom character>"
}
```

Timeseries type (\_key for JSON and couchbase, \_column for CSV)

```json
{
  // timestamp_key for JSON and couchbase, timestamp_column for CSV
  "timestamp_key|timestamp_column": "<name of key/number of column (0-indexed) that contains the timestamp>",
  // value_key for JSON and couchbase, value_column for CSV
  "value_key|value_column": "<name of key/number of column (0-indexed) that contains the value>"
}
```

Table type

```json
{
  "columns": [
    {
      "text": "<column name to reference in bucket, JSON or CSV>",
      "type": "string|number|time"
    }
  ]
}
```

For the /import endpoint, the grafana object can be found from an existing template or from the grafana ui by going to the dashboard, settings, JSON Model and copying all the JSON presented

For the /add endpoint each item in the grafana array should be formatted as follows:

```json
{
  "title": "<title of the panel>",
  // panels are arranged using a 24 column grid system
  "grid_position": {
    "h": "<height of the panel (1-24)>",
    "w": "<width of the panel> (1+)",
    "x": "<start position from the left (0-23)>",
    "y": "<start position from the top (0+)>"
  },
  "type": "graph|gauge|stat|table|text"
}
```

Extra fields are required depending on the type of panel

Every type except text

```json
{
  "targets": "<array of target names specified in data above>",
  // passes straight to grafana (optional) - see example for details
  "overrides": {
    "<field name to override>": {
      "<property to override>": "<value to override property with>"
    }
  }
}
```

Graph type (optional)

```json
{
  // if there are multiple lines on a single graph, this determines whether they stack to show a visual total
  "stack": "true|false",
  // by giving different graphs relative times you can show multiple time periods on the same dashboard
  "relative_time": "<grafana relative time statement>"
}
```

Gauge and stat types (optional)

```json
{
  // the final value that shows up in the stat or gauge is calculated based on the set of datapoints
  "calculation": "last|mean|max|min|first"
}
```

Gauge type

```json
{
  // what value is shown as the maximum possible value on the gauge
  "max": "<max value>",
  // coloured areas for different ranges of the gauge
  "thresholds": {
    "mode": "percentage|absolute",
    "steps": [
      {
        "color": "<name or hex code>",
        "value": "<value at which this threshold has been reached or null for any value>"
      }
    ]
  }
}
```

Text type

```json
{
    // a list of links can be created
  "links": [
    {
      "text": "<what the link is displayed as>",
      "link": "<the link itself>"
    }
  ],
  // plain text formatted as markdown
  "text"
}
```

The response you receive will contain the permalink to the created Grafana dashboard. Dashboards have unique names so creating a dashboard with a duplicate name overrides the existing one

Error response:

```json
{
  "error": ""
}
```

If you are only adding data sources (i.e the grafana array is empty):

```json
{
  "result": "data added"
}
```

Creating a dashboard (i.e. panels defined in the grafana array):

```json
{
  "result": "<grafana permalink>"
}
```

# Example

JSON templates can be found in the templates directory.
server_dynamic_vms.json creates the dashboard found here [here](http://172.23.104.178:3000/d/_kqmMwcMk/server-dynamic-vms?orgId=1&refresh=1m) (VPN required)
