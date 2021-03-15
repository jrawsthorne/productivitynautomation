from couchbase.cluster import Cluster, ClusterOptions
from couchbase.auth import PasswordAuthenticator
from couchbase.options import LockMode
import logging
import json
from flask import Flask, Response

log = logging.getLogger("exporter")
logging.basicConfig(
    format='[%(asctime)s][%(levelname)s] %(message)s', level=logging.DEBUG)

with open("queries.json") as json_file:
    settings = json.load(json_file)

log.info("Loaded queries.json")

log.info("Connecting to clusters")

clusters = {}
for [cluster_name, options] in settings['clusters'].items():
    if cluster_name not in clusters:
        clusters[cluster_name] = Cluster('couchbase://'+options['host'],
                                         ClusterOptions(
            PasswordAuthenticator(options['username'], options['password'])),
            lockmode=LockMode.WAIT)
        log.info("Connected to {}".format(options['host']))

log.info("Connected to clusters")

app = Flask(__name__)

for options in settings['queries']:
    log.info("Registered metrics collection for {}".format(options['name']))

@app.route("/metrics")
def metrics():
    metrics = []
    for options in settings["queries"]:
        log.debug("Collecting metrics for {}".format(options["name"]))
        try:
            rows = clusters[options["cluster"]].query(options["query"]).rows()
            for row in rows:
                if len(options["labels"]) > 0:
                    labels = ["{}=\"{}\"".format(label, row[label]) for label in options["labels"]]
                    metrics.append("{}{{{}}} {}".format(options["name"], ",".join(labels), row[options["value_key"]]))
                else:
                    metrics.append("{} {}".format(options["name"], row[options["value_key"]]))
        except Exception as e:
            log.warning("Error while collecting {}: {}".format(options["name"], e))
    return Response("\n".join(metrics), mimetype="text/plain")


if __name__ == "__main__":
    log.info("Started HTTP server on port 8000")
    app.run(host="0.0.0.0", port=8000)