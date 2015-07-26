package zone

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"sort"
	"strings"
	"time"
)

type Service struct {
	Zone   string
	Server string
}

func ServerPort(server, port string) (string, error) {

	sp := server
	if strings.IndexByte(server, ':') < 0 {
		sp = server + ":" + port
	}

	s, p, err := net.SplitHostPort(sp)
	if err != nil {
		return "", err
	}

	h, err := net.LookupHost(s)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(h[0], p), nil
}

func (s *Service) ServerPort() (string, error) {
	return ServerPort(s.Server, "53")
}

func (s *Service) Transfer() ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetAxfr(s.Zone)

	h, err := s.ServerPort()
	if err != nil {
		return nil, err
	}

	tr := new(dns.Transfer)
	a, err := tr.In(m, h)
	if err != nil {
		return nil, err
	}

	var res []dns.RR
	for ex := range a {
		if ex.Error != nil {
			return nil, ex.Error
		}
		res = append(res, ex.RR...)
	}

	return res, nil
}

func (s *Service) Lookup(name string, record uint16) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), record)
	m.RecursionDesired = true

	h, err := s.ServerPort()
	if err != nil {
		return nil, err
	}

	c := new(dns.Client)
	r, _, err := c.Exchange(m, h)
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, errors.New(fmt.Sprintf("invalid lookup answer for %s", name))
	}

	return r.Answer, nil
}

func copyIP(ip net.IP) net.IP {
	p := make(net.IP, len(ip))
	copy(p, ip)
	return p
}

func (s *Service) Decode(records []dns.RR) *Device {
	d := Device{}

	for _, r := range records {
		d.Name = r.Header().Name
		switch x := r.(type) {
		case *dns.A:
			d.IP = copyIP(x.A)
		case *dns.CNAME:
		case *dns.TXT:
			d.Place = strings.Join(x.Txt, " ")
		case *dns.HINFO:
			d.Code = x.Os
			d.Model = x.Cpu
		case *dns.LOC:
			d.SetLocation(x.Latitude, x.Longitude, x.Altitude)
		}
	}

	return &d
}

func (s *Service) Find(name string) (*Device, error) {
	var res []dns.RR

	// search for an A record
	ans, err := s.Lookup(name, dns.TypeA)
	if err != nil {
		return nil, err
	}
	// we need at least one
	if !(len(ans) > 0) {
		return nil, nil
	}
	res = append(res, ans...)

	// gather other records ...
	txt, err := s.Lookup(name, dns.TypeTXT)
	if err == nil {
		res = append(res, txt...)
	}
	hinfo, err := s.Lookup(name, dns.TypeHINFO)
	if err == nil {
		res = append(res, hinfo...)
	}
	loc, err := s.Lookup(name, dns.TypeLOC)
	if err == nil {
		res = append(res, loc...)
	}

	return s.Decode(res), nil
}

func (s *Service) FindByIP(ip net.IP) (*Device, error) {
	h, err := net.LookupAddr(ip.String())
	if err != nil {
		return nil, err
	}
	if !(len(h) > 0) {
		return nil, nil
	}
	return s.Find(h[0])
}

func (s *Service) List() ([]*Device, error) {

	rr, err := s.Transfer()
	if err != nil {
		return nil, err
	}

	devices := make(map[string]Device)

	// only collect A record details ...
	for _, r := range rr {
		switch x := r.(type) {
		case *dns.A:
			devices[r.Header().Name] = Device{Name: r.Header().Name, IP: x.A}
		}
	}

	// gather other device details ...
	for _, r := range rr {
		d, ok := devices[r.Header().Name]
		if !ok {
			continue
		}
		switch x := r.(type) {
		case *dns.A:
		case *dns.CNAME:
		case *dns.TXT:
			d.Place = strings.Join(x.Txt, " ")
		case *dns.HINFO:
			d.Code = x.Os
			d.Model = x.Cpu
		case *dns.LOC:
			d.SetLocation(x.Latitude, x.Longitude, x.Altitude)
		}
		devices[r.Header().Name] = d
	}

	// sort by device name
	var keys []string
	for k := range devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// build resulting sorted array
	res := make([]*Device, 0, len(devices))
	for _, k := range keys {
		d := devices[k]
		res = append(res, &d)
	}

	return res, nil
}

// see RFC1876 - A Means for Expressing Location Information in the Domain Name System
func cm2size(cms uint32) uint8 {
	var e, v uint32

	for v = cms; v >= 10; v = v / 10 {
		e = e + 1
	}

	return (uint8(v) << 4) | uint8(e&0x0f)
}

// build an OPT RR to allow larger buffer sizes
func (d *Device) ToOPT() *dns.OPT {

	rr := &dns.OPT{
		Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT, Class: dns.ClassINET, Ttl: 0},
	}

	return rr
}

// build a model/code DNS RR
func (d *Device) ToHINFO() *dns.HINFO {

	rr := &dns.HINFO{
		Hdr: dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeHINFO, Class: dns.ClassINET, Ttl: 0},
		Os:  d.Code,
		Cpu: d.Model,
	}

	return rr
}

// build a location DNS RR (use default size etc.)
func (d *Device) ToLOC() *dns.LOC {
	la, lo, a := d.Location()

	rr := &dns.LOC{
		Hdr:       dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeLOC, Class: dns.ClassINET, Ttl: 0},
		Size:      cm2size(10000),
		HorizPre:  cm2size(5000),
		VertPre:   cm2size(5000),
		Latitude:  la,
		Longitude: lo,
		Altitude:  a,
	}

	return rr
}

// build a place name DNS RR
func (d *Device) ToTXT() *dns.TXT {

	rr := &dns.TXT{
		Hdr: dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
		Txt: strings.Split(d.Place, " "),
	}

	return rr
}

// dynamically update the device info stored in DNS
func (s *Service) UpdateInfo(key, secret string, device *Device) error {
	m := new(dns.Msg)

	rr := []dns.RR{
		device.ToOPT(),
		device.ToTXT(),
		device.ToHINFO(),
		device.ToLOC(),
	}

	m.SetUpdate(s.Zone)
	m.SetTsig(dns.Fqdn(key), dns.HmacMD5, 300, time.Now().Unix())
	m.Insert(rr)

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(key): secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid update answer for %s", device.Name))
	}

	return nil
}

// dynamically remove the device info stored in DNS (usually prior to an update)
func (s *Service) RemoveInfo(key, secret string, device *Device) error {
	m := new(dns.Msg)

	rr := []dns.RR{
		device.ToOPT(),
		device.ToTXT(),
		device.ToHINFO(),
		device.ToLOC(),
	}

	m.SetUpdate(s.Zone)
	m.SetTsig(dns.Fqdn(key), dns.HmacMD5, 300, time.Now().Unix())
	m.RemoveRRset(rr)

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(key): secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid update answer for %s", device.Name))
	}

	return nil
}

// remove all RR values stored in DNS
func (s *Service) RemoveAll(key, secret string, device *Device) error {

	rr := &dns.ANY{
		Hdr: dns.RR_Header{Name: dns.Fqdn(device.Name), Rrtype: dns.TypeANY, Class: dns.ClassANY, Ttl: 0},
	}

	m := new(dns.Msg)
	m.SetUpdate(s.Zone)
	m.SetTsig(dns.Fqdn(key), dns.HmacMD5, 300, time.Now().Unix())
	m.RemoveName([]dns.RR{rr})

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(key): secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid update answer for %s", device.Name))
	}

	return nil
}
