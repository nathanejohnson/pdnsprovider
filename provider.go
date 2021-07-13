// Package powerdns implements a powerdns
package powerdns

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/libdns/libdns"
)

// Provider facilitates DNS record manipulation with PowerDNS.
type Provider struct {
	ServerURL string `json:"server_url"`
	ServerID  string `json:"server_id,omitempty"`
	APIToken  string `json:"api_token,omitempty"`
	Debug     string `json:"debug,omitempty"`
	mu        sync.Mutex
	c         *Client
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	prec, err := c.fullZone(ctx, zone)
	if err != nil {
		return nil, err
	}
	recs := make([]libdns.Record, 0, len(prec.ResourceRecordSets))
	for _, rec := range prec.ResourceRecordSets {
		for _, v := range rec.Records {
			recs = append(recs, libdns.Record{
				ID:       prec.ID,
				Type:     rec.Type,
				Name:     rec.Name,
				Value:    v.Content,
				TTL:      time.Second * time.Duration(rec.TTL),
				Priority: 0,
			})
		}
	}
	return recs, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	fullZone, err := c.fullZone(ctx, zone)
	if err != nil {
		return nil, err
	}
	rrecs, err := mergeRRecs(fullZone, records)
	if err != nil {
		return nil, err
	}
	err = c.updateRRs(ctx, fullZone.ID, rrecs)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	zID, err := c.zoneID(ctx, zone)
	if err != nil {
		return nil, err
	}
	inHash := makeLDRecHash(records)
	rRecs := convertLDHash(inHash)
	err = c.updateRRs(ctx, zID, rRecs)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	c, err := p.client()
	if err != nil {
		return nil, err
	}
	fullZone, err := c.fullZone(ctx, zone)
	if err != nil {
		return nil, err
	}

	rRSets := cullRRecs(fullZone, records)
	err = c.updateRRs(ctx, fullZone.ID, rRSets)
	if err != nil {
		return nil, err
	}

	return records, nil

}

func (p *Provider) client() (*Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.c == nil {
		var err error
		if p.ServerID == "" {
			p.ServerID = "localhost"
		}
		var debug io.Writer
		switch strings.ToLower(p.Debug) {
		case "stdout", "yes", "true", "1":
			debug = os.Stdout
		case "stderr":
			debug = os.Stderr
		}
		p.c, err = NewClient(p.ServerID, p.ServerURL, p.APIToken, debug)
		if err != nil {
			return nil, err
		}
	}
	return p.c, nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
