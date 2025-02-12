// Package datadog provides Teamwork-specific integration with Datadog.
package datadog

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
)

var (
	// defaultRate used for metrics.
	defaultRate float64 = 1

	// make sure we implement the Client interface.
	_ Client = &Datadog{}
	_ Client = &NoopClient{}
)

// statsdClient is the interface we use to wrap the datadog statsd type. Useful
// for testing.
type statsdClient interface {
	Gauge(string, float64, []string, float64) error
	Histogram(string, float64, []string, float64) error
	Incr(string, []string, float64) error
	Decr(string, []string, float64) error
	Count(string, int64, []string, float64) error
	Distribution(string, float64, []string, float64) error
}

// ClientOptions are options for the Datadog client.
type ClientOptions struct {
	rate float64
}

// ClientOption is a function that modifies the ClientOptions.
type ClientOption func(*ClientOptions)

// WithRate sets a rate for metrics.
func WithRate(rate float64) ClientOption {
	return func(o *ClientOptions) {
		o.rate = rate
	}
}

// Client is datadog public interface.
type Client interface {
	Gauge(metric string, value float64, tags []string, options ...ClientOption) error
	Histogram(metric string, duration time.Duration, tags []string, options ...ClientOption) error
	Incr(metric string, tags []string, options ...ClientOption) error
	Decr(metric string, tags []string, options ...ClientOption) error
	Count(metric string, count int64, tags []string, options ...ClientOption) error
	Distribution(metric string, value float64, tags []string, options ...ClientOption) error
	Namespace(namespace string, tags []string) Client
}

// Options are options for the Datadog client.
type Options struct {
	defaultRate   float64
	statsdOptions []statsd.Option
}

// Option is a function that modifies the Options.
type Option func(*Options)

// WithDefaultRate sets the default rate for metrics.
func WithDefaultRate(rate float64) Option {
	return func(o *Options) {
		if rate <= 0 || rate > 1 {
			panic(fmt.Sprintf("invalid rate: %f", rate))
		}
		o.defaultRate = rate
	}
}

// WithStatsdOption sets a statsd option.
func WithStatsdOption(option statsd.Option) Option {
	return func(o *Options) {
		o.statsdOptions = append(o.statsdOptions, option)
	}
}

// Datadog is a wrapper around the Datadog statsd client.
type Datadog struct {
	statsd      statsdClient
	defaultRate float64
	namespace   string
	tags        []string
}

// New creates a new Datadog client.
func New(addr string, ns string, tags []string, options ...Option) (*Datadog, error) {
	opts := &Options{
		defaultRate: defaultRate,
	}
	for _, option := range options {
		option(opts)
	}

	statsdClient, err := statsd.New(addr, opts.statsdOptions...)
	if err != nil {
		return nil, err
	}

	return &Datadog{
		statsd:      statsdClient,
		defaultRate: opts.defaultRate,
		namespace:   ns,
		tags:        tags,
	}, nil
}

// Gauge measures the value of a metric at a particular time.
func (d *Datadog) Gauge(metric string, value float64, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Gauge(fmt.Sprintf("%s.%s", d.namespace, metric),
		value, append(d.tags, tags...), opts.rate)
}

// Histogram tracks the statistical distribution of a set of values on each
// host.
func (d *Datadog) Histogram(metric string, duration time.Duration, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Histogram(fmt.Sprintf("%s.%s", d.namespace, metric),
		duration.Seconds()*1000, append(d.tags, tags...), opts.rate)
}

// Incr is just Count of 1.
func (d *Datadog) Incr(metric string, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Incr(fmt.Sprintf("%s.%s", d.namespace, metric),
		append(d.tags, tags...), opts.rate)
}

// Decr is just Count of -1.
func (d *Datadog) Decr(metric string, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Decr(fmt.Sprintf("%s.%s", d.namespace, metric),
		append(d.tags, tags...), opts.rate)
}

// Count tracks how many times something happened per second.
func (d *Datadog) Count(metric string, count int64, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Count(fmt.Sprintf("%s.%s", d.namespace, metric),
		count, append(d.tags, tags...), opts.rate)
}

// Distribution tracks the statistical distribution of a set of values across
// your infrastructure.
func (d *Datadog) Distribution(metric string, value float64, tags []string, options ...ClientOption) error {
	opts := ClientOptions{
		rate: d.defaultRate,
	}
	for _, option := range options {
		option(&opts)
	}
	return d.statsd.Distribution(fmt.Sprintf("%s.%s", d.namespace, metric),
		value, append(d.tags, tags...), opts.rate)
}

// Namespace returns a new Datadog client with the given namespace.
func (d *Datadog) Namespace(namespace string, tags []string) Client {
	return &Datadog{
		statsd:    d.statsd,
		namespace: fmt.Sprintf("%s.%s", d.namespace, namespace),
		tags:      append(d.tags, tags...),
	}
}

// NoopClient is a datadog client that doesn't do anything.
type NoopClient struct{}

// Gauge does nothing.
func (n *NoopClient) Gauge(string, float64, []string, ...ClientOption) error { return nil }

// Histogram does nothing.
func (n *NoopClient) Histogram(string, time.Duration, []string, ...ClientOption) error { return nil }

// Incr does nothing.
func (n *NoopClient) Incr(string, []string, ...ClientOption) error { return nil }

// Decr does nothing.
func (n *NoopClient) Decr(string, []string, ...ClientOption) error { return nil }

// Count does nothing.
func (n *NoopClient) Count(string, int64, []string, ...ClientOption) error { return nil }

// Distribution does nothing.
func (n *NoopClient) Distribution(string, float64, []string, ...ClientOption) error {
	return nil
}

// Namespace does nothing.
func (n *NoopClient) Namespace(string, []string) Client { return &NoopClient{} }
