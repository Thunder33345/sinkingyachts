package sinking_yachts

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
	"strconv"
	"time"
)

//RawClient is the low level api to sinking yachts
//it does not cache and all responses are blocking
//it is safe for concurrent use
type RawClient struct {
	domain      string
	identity    string
	webClient   http.Client
	header      http.Header
	feedTimeout time.Duration
}

//NewRawClient creates a new RawClient
//the endpoint should point to the root of api, without trailing slashes for example "https://example.com" no trailing versions
//the identity is something that identifies your application, and your contact for example "Foo Bot (foobar@example.com)" or "Foo Bot (foobar#12345 on discord)"
//the api may error if identity is incorrect or missing
//webClient is the http.Client that will be used by RawClient, you probably don't want to use default http as there's no timeouts!
//Option is a variadic of optional options to further configure the RawClient
//Note that X-Identity cannot be overwritten with it
func NewRawClient(domain, identity string, webClient http.Client, options ...Option) RawClient {
	h := make(http.Header, 2)
	h.Set("User-Agent", "sinkingyachts/0.1 (https://github.com/Thunder33345/sinkingyachts)")

	client := RawClient{
		domain:      domain,
		identity:    identity,
		webClient:   webClient,
		header:      h,
		feedTimeout: time.Second * 5,
	}
	for _, option := range options {
		option(&client)
	}
	client.header = fixHeaders(client.header, client.identity)
	return client
}

//Feed connects into the wss endpoint to get live updates
//Feed will block forever, and only returns if ctx cancels it, or there's an error
//to cancel use context.WithCancel as ctx
//error will be nil when process exited cleanly
func (c RawClient) Feed(ctx context.Context, modFeed chan DomainUpdate) error {
	var cn *websocket.Conn

	var err error
	opCtx, cancel := context.WithTimeout(ctx, c.feedTimeout)
	cn, _, err = websocket.Dial(opCtx, c.domain+endpointFeed, &websocket.DialOptions{
		HTTPClient: &c.webClient,
		HTTPHeader: c.header,
	})
	cancel()

	if err != nil {
		return err
	}

	defer func() {
		if err == nil || errors.Is(err, ctx.Err()) {
			_ = cn.Close(websocket.StatusNormalClosure, "")
		} else if _, ok := err.(*json.UnmarshalTypeError); ok {
			_ = cn.Close(websocket.StatusInternalError, "invalid json error")
		} else {
			_ = cn.Close(websocket.StatusInternalError, "internal error")
		}
	}()

	for {
		var mod DomainUpdate
		err = wsjson.Read(ctx, cn, &mod)
		if err != nil {
			if errors.Is(err, ctx.Err()) {
				return nil
			}
			return err
		}
		modFeed <- mod
	}
}

//Check will check if a domain is a phishing domain
//true if it's flagged as phishing, false otherwise
func (c RawClient) Check(domain string) (bool, error) {
	resp, err := c.doReq(endpointCheck + domain)
	if err != nil {
		return false, err
	}
	defer closeBody(resp)

	switch resp.StatusCode {
	case 200:
		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		result := string(bytes)
		if result == "true" {
			return true, nil
		} else {
			return false, nil
		}
	default:
		return false, unexpectedStatusError{
			endpoint: c.domain + endpointCheck,
			status:   resp.StatusCode,
		}
	}
}

//All get all phishing domains from the api and return it as a slice of domains
func (c RawClient) All() ([]string, error) {
	resp, err := c.doReq(endpointAll)
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)

	if resp.StatusCode != 200 {
		return nil, unexpectedStatusError{
			endpoint: c.domain + endpointAll,
			status:   resp.StatusCode,
		}
	}
	dec := json.NewDecoder(resp.Body)
	var domains []string
	err = dec.Decode(&domains)
	return domains, err
}

//After will return a slice of changes that are after said time
//the underlying api uses seconds, therefore anything with finer will be rounded up
func (c RawClient) After(after time.Time) ([]DomainUpdate, error) {
	return c.Recent(int(math.Ceil(time.Since(after).Seconds())))
}

//Recent returns changes that are recently done in given seconds
//Changes will be represented as DomainUpdate
func (c RawClient) Recent(seconds int) ([]DomainUpdate, error) {
	resp, err := c.doReq(endpointRecent + strconv.Itoa(seconds))
	if err != nil {
		return nil, err
	}
	defer closeBody(resp)

	if resp.StatusCode != 200 {
		return nil, unexpectedStatusError{
			endpoint: c.domain + endpointRecent,
			status:   resp.StatusCode,
		}
	}
	dec := json.NewDecoder(resp.Body)
	var mods []DomainUpdate
	err = dec.Decode(&mods)
	return mods, err
}

//Size returns the total amount of domains that are stored
func (c RawClient) Size() (int, error) {
	resp, err := c.doReq(endpointSize)
	if err != nil {
		return 0, err
	}
	defer closeBody(resp)

	if resp.StatusCode != 200 {
		return 0, unexpectedStatusError{
			endpoint: c.domain + endpointAll,
			status:   resp.StatusCode,
		}
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(bytes))
}

func (c RawClient) doReq(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.domain+endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.header
	return c.webClient.Do(req)
}

//fixHeaders is an internal function that returns a cloned header if given header not nil
//it also overwrites "X-Identity" to a given identity
func fixHeaders(header http.Header, identity string) http.Header {
	var h http.Header
	if header != nil {
		h = header.Clone()
	} else {
		h = make(http.Header, 1)
	}
	h.Set("X-Identity", identity)
	return h
}

func closeBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
