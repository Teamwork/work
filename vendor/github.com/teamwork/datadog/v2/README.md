[![GoDoc](https://godoc.org/github.com/Teamwork/spamc?status.svg)](https://tw-godoc.teamwork.com/datadog/)

Datadog provides a common interface to report metrics to Datadog in an orderly
manner.

## Usage

```go
dd, err := datadog.New("localhost:8125", "my-app", []string{"containerid:abz123"})
if err != nil {
	panic(err.Error())
}

dd.Histogram("duration", 10*time.Second, []string{"auth:true"})
```
