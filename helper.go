package sinking_yachts

import (
	"context"
	"io"
	"io/ioutil"
	"time"
)

//ReadCacheFrom loads stored cache from the reader into Client
func ReadCacheFrom(c *Client, r io.Reader) error {
	bf, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	var data Client
	err = data.UnmarshalJSON(bf)
	if err != nil {
		return err
	}

	c.m.Lock()
	defer c.m.Unlock()
	c.lastUpdated = data.lastUpdated
	c.domains = data.domains
	return nil
}

//WriteCacheInto saves cache into the writer.
func WriteCacheInto(c *Client, w io.Writer) error {
	if s, ok := w.(io.Seeker); ok {
		_, err := s.Seek(0, 0)
		if err != nil {
			return err
		}
	}
	b, err := c.MarshalJSON()

	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

//SaveOnChange register listen for updates and writes it into the writer
//this function blocks and returns only when update channel gets closed, use ctx to cancel
func SaveOnChange(ctx context.Context, c *Client, w io.Writer) error {
	ch := c.UpdateChannel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-ch:
			if !ok {
				return nil
			}
			err := WriteCacheInto(c, w)
			if err != nil {
				return err
			}
		}
	}
}

//AutoSync is a helper that setup auto syncing functionality for Client
//this function blocks and return only when cancelled by ctx, or occurrence of an error
//it's recommended to use full sync, especially when realtime is enabled
func AutoSync(ctx context.Context, c *Client, realtime bool, recentInterval, fullSyncInterval time.Duration) error {
	var stream chan error
	modChan := make(chan DomainUpdate, 2)
	if realtime {
		go func() {
			stream <- c.listenForUpdates(ctx, modChan)
			close(stream)
		}()
	}

	errSync := c.FullSync()
	if errSync != nil {
		return errSync
	}

	var recentTicker *time.Ticker
	var fullSyncTicker *time.Ticker
	if recentInterval > 0 {
		recentTicker = time.NewTicker(recentInterval)
	}
	if fullSyncInterval > 0 {
		fullSyncTicker = time.NewTicker(fullSyncInterval)
	}
	defer recentTicker.Stop()
	defer fullSyncTicker.Stop()
	for {
		select {
		case mod, ok := <-modChan:
			if !ok {
				return nil
			}
			c.applyLiveUpdates(mod)
		case <-recentTicker.C:
			err := c.Update()
			if err != nil {
				return err
			}
		case <-fullSyncTicker.C:
			err := c.FullSync()
			if err != nil {
				return err
			}
		case err := <-stream:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
