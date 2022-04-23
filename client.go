package sinkingyachts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	r           RawClient
	domains     map[string]empty
	lastUpdated time.Time
	m           sync.Mutex
	streaming   bool
	cancelFunc  context.CancelFunc
	updateChan  chan struct{}
}

func New(endpoint, identity string, client http.Client, options ...Option) *Client {
	api := &Client{
		r:       NewRawClient(endpoint, identity, client, options...),
		domains: map[string]empty{},
	}

	return api
}

//Check if a domain is phishing
//parent domains will not be checked, FuzzyCheck should be used instead
func (c *Client) Check(domain string) bool {
	c.m.Lock()
	defer c.m.Unlock()
	_, found := c.domains[domain]
	return found
}

//FuzzyCheck if a domain is phishing
//fuzzy check includes checking parent domains (foo.bar.bad.com will check bar.bad.com and bad.com)
//and returns true if any of the domains is phishing
func (c *Client) FuzzyCheck(domain string) bool {
	for _, part := range generateVariants(domain) {
		if c.Check(part) {
			return true
		}
	}
	return false
}

//Domains return a list of known phishing domains.
//there are no specific order of the domains.
func (c *Client) Domains() []string {
	c.m.Lock()
	defer c.m.Unlock()
	domains := make([]string, 0, len(c.domains))
	for domain := range c.domains {
		domains = append(domains, domain)
	}
	return domains
}

//Size return the amount of known phishing domains.
func (c *Client) Size() int {
	c.m.Lock()
	defer c.m.Unlock()
	return len(c.domains)
}

//FullSync clears the local cache and loading all known domain form the api
func (c *Client) FullSync() error {
	ds, err := c.r.All()
	if err != nil {
		return err
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.lastUpdated = time.Now()
	dMap := map[string]empty{}
	for _, d := range ds {
		dMap[d] = empty{}
	}
	c.domains = dMap
	c.sendUpdate()
	return nil
}

//Update updates the list of known phishing domains from the api based on last update time.
func (c *Client) Update() error {
	c.m.Lock()
	defer c.m.Unlock()
	mods, err := c.r.After(c.lastUpdated.Add(-(time.Minute * 1)))
	if err != nil {
		return err
	}
	c.lastUpdated = time.Now()
	for _, mod := range mods {
		c.applyMod(mod)
	}
	if len(mods) > 0 {
		c.sendUpdate()
	}
	return nil
}

//ListenForUpdates starts a wss connection to the api and listens for updates.
//use ctx to cancel close the connection
func (c *Client) ListenForUpdates(ctx context.Context) error {

	modChan := make(chan DomainUpdate, 8)
	defer close(modChan)
	go func(a *Client) {
		for {
			select {
			case <-ctx.Done():
				return
			case mod, ok := <-modChan:
				if !ok {
					return
				}
				a.applyLiveUpdates(mod)
			}
		}
	}(c)

	return c.listenForUpdates(ctx, modChan)
}

//applyLiveUpdates applies an update to the cache
func (c *Client) applyLiveUpdates(mod DomainUpdate) {
	c.m.Lock()
	defer c.m.Unlock()
	c.lastUpdated = time.Now()
	c.applyMod(mod)
	c.sendUpdate()
}

//listenForUpdates listens for updates from the api and pipe it into modChan
func (c *Client) listenForUpdates(ctx context.Context, modChan chan DomainUpdate) error {
	checkStreaming := func() error {
		c.m.Lock()
		defer c.m.Unlock()
		if c.streaming {
			return fmt.Errorf("already listening for updates")
		}
		c.streaming = true
		ctx, c.cancelFunc = context.WithCancel(ctx)
		return nil
	}
	if err := checkStreaming(); err != nil {
		return err
	}

	defer func() {
		c.m.Lock()
		defer c.m.Unlock()
		c.streaming = false
		if c.cancelFunc != nil {
			c.cancelFunc()
		}
		c.cancelFunc = nil
	}()
	return c.r.Feed(ctx, modChan)
}

//Close closes the client and releases all resources.
func (c *Client) Close() error {
	c.m.Lock()
	defer c.m.Unlock()
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	c.domains = nil
	close(c.updateChan)
	c.updateChan = nil
	return nil
}

//Raw returns the underlying api client.
func (c *Client) Raw() RawClient {
	return c.r
}

//UpdateChannel returns a channel that emits empty struct whenever Client's domain get updated
//calls will unregister the previous channel
//update may get dropped if channel is full, sends do not wait for receiver
func (c *Client) UpdateChannel() chan struct{} {
	if c.updateChan != nil {
		close(c.updateChan)
	}

	c.updateChan = make(chan struct{}, 2)
	return c.updateChan
}

//sendUpdate emits into the update chan if it is set
//should only be called when mutex is locked
func (c *Client) sendUpdate() {
	if c.updateChan == nil {
		return
	}
	select {
	case c.updateChan <- struct{}{}:
	default:
	}
}

//applyMod applies an update to the cache
//should only be called when mutex is locked
func (c *Client) applyMod(mod DomainUpdate) {
	for _, domain := range mod.Domains {
		if mod.Add {
			c.domains[domain] = empty{}
		} else {
			delete(c.domains, domain)
		}
	}
}

//MarshalJSON marshal the Client's cache to JSON
func (c *Client) MarshalJSON() ([]byte, error) {
	c.m.Lock()
	defer c.m.Unlock()
	sf := save{
		LastUpdated: c.lastUpdated,
		Domains:     make([]string, 0, len(c.domains)),
	}
	for d := range c.domains {
		sf.Domains = append(sf.Domains, d)
	}
	return json.Marshal(sf)
}

//UnmarshalJSON unmarshal the Client's cache from JSON
func (c *Client) UnmarshalJSON(data []byte) error {
	var sf save
	err := json.Unmarshal(data, &sf)
	if err != nil {
		return err
	}
	c.lastUpdated = sf.LastUpdated
	dMap := map[string]empty{}
	for _, d := range sf.Domains {
		dMap[d] = empty{}
	}
	c.domains = dMap
	return nil
}

//generateVariants generate variations of the domain and parent domains
//"foo.bar.bad.com" will generate itself, "bar.bad.com" and "bad.com" but not "com"
//could have been optimized with callbacks or channels but this is simpler
func generateVariants(domain string) []string {
	var variants []string
	parts := strings.Split(domain, ".")
	for i := 0; i < len(parts)-1; i++ {
		variants = append(variants, strings.Join(parts[i:], "."))
	}
	return variants
}
