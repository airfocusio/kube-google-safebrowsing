# kube-google-safebrowsing

A Kubernetes Deployment that continuously watches ingresses for problems via [Googles Safebrowsing API](https://developers.google.com/safe-browsing/v4) and exposes this information as Prometheus metrics.

## Testing

```bash
# start local kind cluster
make test-deps

# run tests
make test
```

## Metrics

* `google_safebrowsing_threat_matches`: Gauge that is either `0` or `1` if there was a threat found

## Labels

* `domain`: The domain that has been checked

## Query

TODO
