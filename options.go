package sinkingyachts

import (
	"net/http"
	"time"
)

//Option is a function that can configure a RawClient
//Option should only be used by NewRawClient, using it in any other way may risk race error and undefined behaviour
type Option func(client *RawClient)

//WithHeaders sets a custom header to RawClient, if "X-Identity" is present, it will be overwritten by RawClient's identity
func WithHeaders(header http.Header) Option {
	return func(client *RawClient) {
		client.header = fixHeaders(header, client.identity)
	}
}

func WithHeader(key string, value string) Option {
	return func(client *RawClient) {
		client.header.Set(key, value)
	}
}

func WithoutHeader(header string) Option {
	return func(client *RawClient) {
		client.header.Del(header)
	}
}

//WithFeedTimeout sets a custom feed timeout for dialing to the websocket update feed
func WithFeedTimeout(duration time.Duration) Option {
	return func(client *RawClient) {
		client.feedTimeout = duration
	}
}
