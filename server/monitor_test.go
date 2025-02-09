// Copyright 2015 Apcera Inc. All rights reserved.

package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats"
)

const CLIENT_PORT = 11224
const MONITOR_PORT = 11424
const CLUSTER_PORT = 12444

var DefaultMonitorOptions = Options{
	Host:        "localhost",
	Port:        CLIENT_PORT,
	HTTPPort:    MONITOR_PORT,
	ClusterHost: "localhost",
	ClusterPort: CLUSTER_PORT,
	NoLog:       true,
	NoSigs:      true,
}

func runMonitorServer() *Server {
	resetPreviousHTTPConnections()
	opts := DefaultMonitorOptions
	return RunServer(&opts)
}

func runMonitorServerNoHTTPPort() *Server {
	resetPreviousHTTPConnections()
	opts := DefaultMonitorOptions
	opts.HTTPPort = 0
	return RunServer(&opts)
}

func resetPreviousHTTPConnections() {
	http.DefaultTransport = &http.Transport{}
}

func TestMyUptime(t *testing.T) {
	// Make sure we print this stuff right.
	var d time.Duration
	var s string

	d = 22 * time.Second
	s = myUptime(d)
	if s != "22s" {
		t.Fatalf("Expected `22s`, go ``%s`", s)
	}
	d = 4*time.Minute + d
	s = myUptime(d)
	if s != "4m22s" {
		t.Fatalf("Expected `4m22s`, go ``%s`", s)
	}
	d = 4*time.Hour + d
	s = myUptime(d)
	if s != "4h4m22s" {
		t.Fatalf("Expected `4h4m22s`, go ``%s`", s)
	}
	d = 32*24*time.Hour + d
	s = myUptime(d)
	if s != "32d4h4m22s" {
		t.Fatalf("Expected `32d4h4m22s`, go ``%s`", s)
	}
	d = 22*365*24*time.Hour + d
	s = myUptime(d)
	if s != "22y32d4h4m22s" {
		t.Fatalf("Expected `22y32d4h4m22s`, go ``%s`", s)
	}
}

// Make sure that we do not run the http server for monitoring unless asked.
func TestNoMonitorPort(t *testing.T) {
	s := runMonitorServerNoHTTPPort()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	if resp, err := http.Get(url + "varz"); err == nil {
		t.Fatalf("Expected error: Got %+v\n", resp)
	}
	if resp, err := http.Get(url + "healthz"); err == nil {
		t.Fatalf("Expected error: Got %+v\n", resp)
	}
	if resp, err := http.Get(url + "connz"); err == nil {
		t.Fatalf("Expected error: Got %+v\n", resp)
	}
}

func TestVarz(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "varz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Expected application/json content-type, got %s\n", ct)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	v := Varz{}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	// Do some sanity checks on values
	if time.Since(v.Start) > 10*time.Second {
		t.Fatal("Expected start time to be within 10 seconds.")
	}

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	resp, err = http.Get(url + "varz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	v = Varz{}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if v.Connections != 1 {
		t.Fatalf("Expected Connections of 1, got %v\n", v.Connections)
	}
	if v.TotalConnections < 1 {
		t.Fatalf("Expected Total Connections of at least 1, got %v\n", v.TotalConnections)
	}
	if v.InMsgs != 1 {
		t.Fatalf("Expected InMsgs of 1, got %v\n", v.InMsgs)
	}
	if v.OutMsgs != 1 {
		t.Fatalf("Expected OutMsgs of 1, got %v\n", v.OutMsgs)
	}
	if v.InBytes != 5 {
		t.Fatalf("Expected InBytes of 5, got %v\n", v.InBytes)
	}
	if v.OutBytes != 5 {
		t.Fatalf("Expected OutBytes of 5, got %v\n", v.OutBytes)
	}

	// Test JSONP
	respj, errj := http.Get(fmt.Sprintf("http://localhost:%d/", MONITOR_PORT) + "varz?callback=callback")
	if errj != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	ct = respj.Header.Get("Content-Type")
	if ct != "application/javascript" {
		t.Fatalf("Expected application/javascript content-type, got %s\n", ct)
	}
	defer respj.Body.Close()
}

func TestConnz(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Expected application/json content-type, got %s\n", ct)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	// Test contents..
	if c.NumConns != 0 {
		t.Fatalf("Expected 0 connections, got %d\n", c.NumConns)
	}
	if c.Conns == nil || len(c.Conns) != 0 {
		t.Fatalf("Expected 0 connections in array, got %p\n", c.Conns)
	}

	// Test with connections.
	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	resp, err = http.Get(url + "connz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.NumConns != 1 {
		t.Fatalf("Expected 1 connections, got %d\n", c.NumConns)
	}
	if c.Conns == nil || len(c.Conns) != 1 {
		t.Fatalf("Expected 1 connections in array, got %d\n", len(c.Conns))
	}

	if c.Limit != DefaultConnListSize {
		t.Fatalf("Expected limit of %d, got %v\n", DefaultConnListSize, c.Limit)
	}

	if c.Offset != 0 {
		t.Fatalf("Expected offset of 0, got %v\n", c.Offset)
	}

	// Test inside details of each connection
	ci := c.Conns[0]

	if ci.Cid == 0 {
		t.Fatalf("Expected non-zero cid, got %v\n", ci.Cid)
	}
	if ci.IP != "127.0.0.1" {
		t.Fatalf("Expected \"127.0.0.1\" for IP, got %v\n", ci.IP)
	}
	if ci.Port == 0 {
		t.Fatalf("Expected non-zero port, got %v\n", ci.Port)
	}
	if ci.NumSubs != 1 {
		t.Fatalf("Expected num_subs of 1, got %v\n", ci.NumSubs)
	}
	if len(ci.Subs) != 0 {
		t.Fatalf("Expected subs of 0, got %v\n", ci.Subs)
	}
	if ci.InMsgs != 1 {
		t.Fatalf("Expected InMsgs of 1, got %v\n", ci.InMsgs)
	}
	if ci.OutMsgs != 1 {
		t.Fatalf("Expected OutMsgs of 1, got %v\n", ci.OutMsgs)
	}
	if ci.InBytes != 5 {
		t.Fatalf("Expected InBytes of 1, got %v\n", ci.InBytes)
	}
	if ci.OutBytes != 5 {
		t.Fatalf("Expected OutBytes of 1, got %v\n", ci.OutBytes)
	}
	if ci.Start.IsZero() {
		t.Fatalf("Expected Start to be valid\n")
	}
	if ci.Uptime == "" {
		t.Fatalf("Expected Uptime to be valid\n")
	}
	if ci.LastActivity.IsZero() {
		t.Fatalf("Expected LastActivity to be valid\n")
	}
	if ci.LastActivity.UnixNano() < ci.Start.UnixNano() {
		t.Fatalf("Expected LastActivity [%v] to be > Start [%v]\n", ci.LastActivity, ci.Start)
	}
	if ci.Idle == "" {
		t.Fatalf("Expected Idle to be valid\n")
	}

	// Test JSONP
	respj, errj := http.Get(fmt.Sprintf("http://localhost:%d/", MONITOR_PORT) + "connz?callback=callback")
	if errj != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	ct = respj.Header.Get("Content-Type")
	if ct != "application/javascript" {
		t.Fatalf("Expected application/javascript content-type, got %s\n", ct)
	}
	defer respj.Body.Close()
}

func TestConnzWithSubs(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?subs=1")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	// Test inside details of each connection
	ci := c.Conns[0]
	if len(ci.Subs) != 1 || ci.Subs[0] != "foo" {
		t.Fatalf("Expected subs of 1, got %v\n", ci.Subs)
	}
}

func TestConnzLastActivity(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	nc2 := createClientConnSubscribeAndPublish(t)
	defer nc2.Close()

	pollConz := func() *Connz {
		url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
		resp, err := http.Get(url + "connz?subs=1")
		if err != nil {
			t.Fatalf("Expected no error: Got %v\n", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Got an error reading the body: %v\n", err)
		}
		c := Connz{}
		if err := json.Unmarshal(body, &c); err != nil {
			t.Fatalf("Got an error unmarshalling the body: %v\n", err)
		}
		return &c
	}

	// Test inside details of each connection
	ci := pollConz().Conns[0]
	if len(ci.Subs) != 1 {
		t.Fatalf("Expected subs of 1, got %v\n", len(ci.Subs))
	}
	firstLast := ci.LastActivity
	if firstLast.IsZero() {
		t.Fatalf("Expected LastActivity to be valid\n")
	}

	// On Windows, looks like the precision is too low, and if we
	// don't wait, first and last would be equal.
	if runtime.GOOS == "windows" {
		time.Sleep(100 * time.Millisecond)
	}

	// Sub should trigger update.
	nc.Subscribe("hello.world", func(m *nats.Msg) {})
	nc.Flush()
	ci = pollConz().Conns[0]
	subLast := ci.LastActivity
	if firstLast.Equal(subLast) {
		t.Fatalf("Subscribe should have triggered update to LastActivity\n")
	}

	// On Windows, looks like the precision is too low, and if we
	// don't wait, first and last would be equal.
	if runtime.GOOS == "windows" {
		time.Sleep(100 * time.Millisecond)
	}

	// Pub should trigger as well
	nc.Publish("foo", []byte("Hello"))
	nc.Flush()
	ci = pollConz().Conns[0]
	pubLast := ci.LastActivity
	if subLast.Equal(pubLast) {
		t.Fatalf("Publish should have triggered update to LastActivity\n")
	}

	// On Windows, looks like the precision is too low, and if we
	// don't wait, first and last would be equal.
	if runtime.GOOS == "windows" {
		time.Sleep(100 * time.Millisecond)
	}

	// Message delivery should trigger as well
	nc2.Publish("foo", []byte("Hello"))
	nc2.Flush()
	nc.Flush()
	ci = pollConz().Conns[0]
	msgLast := ci.LastActivity
	if pubLast.Equal(msgLast) {
		t.Fatalf("Message delivery should have triggered update to LastActivity\n")
	}
}

func TestConnzWithOffsetAndLimit(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)

	// Test that offset and limit ok when not enough connections
	resp, err := http.Get(url + "connz?offset=1&limit=1")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}
	if c.Conns == nil || len(c.Conns) != 0 {
		t.Fatalf("Expected 0 connections in array, got %p\n", c.Conns)
	}

	cl1 := createClientConnSubscribeAndPublish(t)
	defer cl1.Close()

	cl2 := createClientConnSubscribeAndPublish(t)
	defer cl2.Close()

	resp, err = http.Get(url + "connz?offset=1&limit=1")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c = Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Limit != 1 {
		t.Fatalf("Expected limit of 1, got %v\n", c.Limit)
	}

	if c.Offset != 1 {
		t.Fatalf("Expected offset of 1, got %v\n", c.Offset)
	}

	if len(c.Conns) != 1 {
		t.Fatalf("Expected conns of 1, got %v\n", len(c.Conns))
	}

	if c.NumConns != 1 {
		t.Fatalf("Expected NumConns to be 1, got %v\n", c.NumConns)
	}

	resp, err = http.Get(url + "connz?offset=2&limit=1")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c = Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Limit != 1 {
		t.Fatalf("Expected limit of 1, got %v\n", c.Limit)
	}

	if c.Offset != 2 {
		t.Fatalf("Expected offset of 2, got %v\n", c.Offset)
	}

	if len(c.Conns) != 0 {
		t.Fatalf("Expected conns of 0, got %v\n", len(c.Conns))
	}

	if c.NumConns != 0 {
		t.Fatalf("Expected NumConns to be 0, got %v\n", c.NumConns)
	}
}

func TestConnzDefaultSorted(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	clients := make([]*nats.Conn, 4)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].Cid > c.Conns[1].Cid ||
		c.Conns[1].Cid > c.Conns[2].Cid ||
		c.Conns[2].Cid > c.Conns[3].Cid {
		t.Fatalf("Expected conns sorted in ascending order by cid, got %v < %v\n", c.Conns[0].Cid, c.Conns[3].Cid)
	}
}

func TestConnzSortedByCid(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	clients := make([]*nats.Conn, 4)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=cid")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].Cid > c.Conns[1].Cid ||
		c.Conns[1].Cid > c.Conns[2].Cid ||
		c.Conns[2].Cid > c.Conns[3].Cid {
		t.Fatalf("Expected conns sorted in ascending order by cid, got %v < %v\n", c.Conns[0].Cid, c.Conns[3].Cid)
	}
}

func TestConnzSortedByBytesAndMsgs(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	// Create a connection and make it send more messages than others
	firstClient := createClientConnSubscribeAndPublish(t)
	for i := 0; i < 100; i++ {
		firstClient.Publish("foo", []byte("Hello World"))
	}
	defer firstClient.Close()
	firstClient.Flush()

	clients := make([]*nats.Conn, 3)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=bytes_to")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].OutBytes < c.Conns[1].OutBytes ||
		c.Conns[0].OutBytes < c.Conns[2].OutBytes ||
		c.Conns[0].OutBytes < c.Conns[3].OutBytes {
		t.Fatalf("Expected conns sorted in descending order by bytes to, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].OutBytes, c.Conns[1].OutBytes, c.Conns[2].OutBytes, c.Conns[3].OutBytes)
	}

	url = fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err = http.Get(url + "connz?sort=msgs_to")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c = Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].OutMsgs < c.Conns[1].OutMsgs ||
		c.Conns[0].OutMsgs < c.Conns[2].OutMsgs ||
		c.Conns[0].OutMsgs < c.Conns[3].OutMsgs {
		t.Fatalf("Expected conns sorted in descending order by msgs from, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].OutMsgs, c.Conns[1].OutMsgs, c.Conns[2].OutMsgs, c.Conns[3].OutMsgs)
	}

	url = fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err = http.Get(url + "connz?sort=bytes_from")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c = Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].InBytes < c.Conns[1].InBytes ||
		c.Conns[0].InBytes < c.Conns[2].InBytes ||
		c.Conns[0].InBytes < c.Conns[3].InBytes {
		t.Fatalf("Expected conns sorted in descending order by bytes from, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].InBytes, c.Conns[1].InBytes, c.Conns[2].InBytes, c.Conns[3].InBytes)
	}

	url = fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err = http.Get(url + "connz?sort=msgs_from")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c = Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].InMsgs < c.Conns[1].InMsgs ||
		c.Conns[0].InMsgs < c.Conns[2].InMsgs ||
		c.Conns[0].InMsgs < c.Conns[3].InMsgs {
		t.Fatalf("Expected conns sorted in descending order by msgs from, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].InMsgs, c.Conns[1].InMsgs, c.Conns[2].InMsgs, c.Conns[3].InMsgs)
	}
}

func TestConnzSortedByPending(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	firstClient := createClientConnSubscribeAndPublish(t)
	firstClient.Subscribe("hello.world", func(m *nats.Msg) {})
	clients := make([]*nats.Conn, 3)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}
	defer firstClient.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=pending")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].Pending < c.Conns[1].Pending ||
		c.Conns[0].Pending < c.Conns[2].Pending ||
		c.Conns[0].Pending < c.Conns[3].Pending {
		t.Fatalf("Expected conns sorted in descending order by number of pending, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].Pending, c.Conns[1].Pending, c.Conns[2].Pending, c.Conns[3].Pending)
	}
}

func TestConnzSortedBySubs(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	firstClient := createClientConnSubscribeAndPublish(t)
	firstClient.Subscribe("hello.world", func(m *nats.Msg) {})
	clients := make([]*nats.Conn, 3)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}
	defer firstClient.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=subs")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].NumSubs < c.Conns[1].NumSubs ||
		c.Conns[0].NumSubs < c.Conns[2].NumSubs ||
		c.Conns[0].NumSubs < c.Conns[3].NumSubs {
		t.Fatalf("Expected conns sorted in descending order by number of subs, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].NumSubs, c.Conns[1].NumSubs, c.Conns[2].NumSubs, c.Conns[3].NumSubs)
	}
}

func TestConnzSortedByLast(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	firstClient := createClientConnSubscribeAndPublish(t)
	defer firstClient.Close()
	firstClient.Subscribe("hello.world", func(m *nats.Msg) {})
	firstClient.Flush()

	clients := make([]*nats.Conn, 3)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
		clients[i].Flush()
	}

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=last")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if c.Conns[0].LastActivity.UnixNano() < c.Conns[1].LastActivity.UnixNano() ||
		c.Conns[1].LastActivity.UnixNano() < c.Conns[2].LastActivity.UnixNano() ||
		c.Conns[2].LastActivity.UnixNano() < c.Conns[3].LastActivity.UnixNano() {
		t.Fatalf("Expected conns sorted in descending order by lastActivity, got %v < one of [%v, %v, %v]\n",
			c.Conns[0].LastActivity, c.Conns[1].LastActivity, c.Conns[2].LastActivity, c.Conns[3].LastActivity)
	}
}

func TestConnzSortedByIdle(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	firstClient := createClientConnSubscribeAndPublish(t)
	defer firstClient.Close()
	firstClient.Subscribe("client.1", func(m *nats.Msg) {})
	firstClient.Flush()

	secondClient := createClientConnSubscribeAndPublish(t)
	defer secondClient.Close()
	secondClient.Subscribe("client.2", func(m *nats.Msg) {})
	secondClient.Flush()

	// The Idle granularity is a whole second
	time.Sleep(time.Second)
	firstClient.Publish("client.1", []byte("new message"))

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=idle")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	// Make sure we are returned 2 connections...
	if len(c.Conns) != 2 {
		t.Fatalf("Expected to get two connections, got %v", len(c.Conns))
	}

	// And that the Idle time is valid (even if equal to "0s")
	if c.Conns[0].Idle == "" || c.Conns[1].Idle == "" {
		t.Fatal("Expected Idle value to be valid")
	}

	idle1, err := time.ParseDuration(c.Conns[0].Idle)
	if err != nil {
		t.Fatalf("Unable to parse duration %v, err=%v", c.Conns[0].Idle, err)
	}
	idle2, err := time.ParseDuration(c.Conns[1].Idle)
	if err != nil {
		t.Fatalf("Unable to parse duration %v, err=%v", c.Conns[0].Idle, err)
	}

	if idle1 < idle2 {
		t.Fatalf("Expected conns sorted in descending order by Idle, got %v < %v\n",
			idle1, idle2)
	}
}

func TestConnzSortBadRequest(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	firstClient := createClientConnSubscribeAndPublish(t)
	firstClient.Subscribe("hello.world", func(m *nats.Msg) {})
	clients := make([]*nats.Conn, 3)
	for i := range clients {
		clients[i] = createClientConnSubscribeAndPublish(t)
		defer clients[i].Close()
	}
	defer firstClient.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz?sort=foo")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("Expected a 400 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
}

func TestConnzWithRoutes(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	var opts = Options{
		Host:        "localhost",
		Port:        CLIENT_PORT + 1,
		ClusterHost: "localhost",
		ClusterPort: CLUSTER_PORT + 1,
		NoLog:       true,
		NoSigs:      true,
	}
	routeURL, _ := url.Parse(fmt.Sprintf("nats-route://localhost:%d", CLUSTER_PORT))
	opts.Routes = []*url.URL{routeURL}

	sc := RunServer(&opts)
	defer sc.Shutdown()

	time.Sleep(time.Second)

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Expected application/json content-type, got %s\n", ct)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	// Test contents..
	// Make sure routes don't show up under connz, but do under routez
	if c.NumConns != 0 {
		t.Fatalf("Expected 0 connections, got %d\n", c.NumConns)
	}
	if c.Conns == nil || len(c.Conns) != 0 {
		t.Fatalf("Expected 0 connections in array, got %p\n", c.Conns)
	}

	// Now check routez
	url = fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err = http.Get(url + "routez")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	rz := Routez{}
	if err := json.Unmarshal(body, &rz); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if rz.NumRoutes != 1 {
		t.Fatalf("Expected 1 route, got %d\n", rz.NumRoutes)
	}

	if len(rz.Routes) != 1 {
		t.Fatalf("Expected route array of 1, got %v\n", len(rz.Routes))
	}

	route := rz.Routes[0]

	if route.DidSolicit {
		t.Fatalf("Expected unsolicited route, got %v\n", route.DidSolicit)
	}

	// Test JSONP
	respj, errj := http.Get(fmt.Sprintf("http://localhost:%d/", MONITOR_PORT) + "routez?callback=callback")
	if errj != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	ct = respj.Header.Get("Content-Type")
	if ct != "application/javascript" {
		t.Fatalf("Expected application/javascript content-type, got %s\n", ct)
	}
	defer respj.Body.Close()
}

func TestSubsz(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "subscriptionsz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Expected application/json content-type, got %s\n", ct)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}
	sl := Subsz{}
	if err := json.Unmarshal(body, &sl); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	if sl.NumSubs != 1 {
		t.Fatalf("Expected NumSubs of 1, got %d\n", sl.NumSubs)
	}
	if sl.NumInserts != 1 {
		t.Fatalf("Expected NumInserts of 1, got %d\n", sl.NumInserts)
	}
	if sl.NumMatches != 1 {
		t.Fatalf("Expected NumMatches of 1, got %d\n", sl.NumMatches)
	}

	// Test JSONP
	respj, errj := http.Get(fmt.Sprintf("http://localhost:%d/", MONITOR_PORT) + "subscriptionsz?callback=callback")
	ct = respj.Header.Get("Content-Type")
	if errj != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if ct != "application/javascript" {
		t.Fatalf("Expected application/javascript content-type, got %s\n", ct)
	}
	defer respj.Body.Close()
}

// Tests handle root
func TestHandleRoot(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/", MONITOR_PORT))
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("Expected text/html response, got %s\n", ct)
	}
	defer resp.Body.Close()
}

func TestConnzWithNamedClient(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	clientName := "test-client"
	nc := createClientConnWithName(t, clientName)
	defer nc.Close()

	url := fmt.Sprintf("http://localhost:%d/", MONITOR_PORT)
	resp, err := http.Get(url + "connz")
	if err != nil {
		t.Fatalf("Expected no error: Got %v\n", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("Expected application/json content-type, got %s\n", ct)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Got an error reading the body: %v\n", err)
	}

	// Confirm server is exposing client name in monitoring endpoint.
	c := Connz{}
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatalf("Got an error unmarshalling the body: %v\n", err)
	}

	got := len(c.Conns)
	expected := 1
	if got != expected {
		t.Fatalf("Expected %d connection in array, got %d\n", expected, got)
	}

	conn := c.Conns[0]
	if conn.Name != clientName {
		t.Fatalf("Expected client to have name %q. got %q", clientName, conn.Name)
	}
}

// Create a connection to test ConnInfo
func createClientConnSubscribeAndPublish(t *testing.T) *nats.Conn {
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", CLIENT_PORT))
	if err != nil {
		t.Fatalf("Error creating client: %v\n", err)
	}

	ch := make(chan bool)
	nc.Subscribe("foo", func(m *nats.Msg) { ch <- true })
	nc.Publish("foo", []byte("Hello"))
	// Wait for message
	<-ch
	return nc
}

func createClientConnWithName(t *testing.T, name string) *nats.Conn {
	natsURI := fmt.Sprintf("nats://localhost:%d", CLIENT_PORT)

	client := nats.DefaultOptions
	client.Servers = []string{natsURI}
	client.Name = name
	nc, err := client.Connect()
	if err != nil {
		t.Fatalf("Error creating client: %v\n", err)
	}

	return nc
}
