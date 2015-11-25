// Copyright 2012 Junqing Tan <ivan@mysqlab.net> and The Go Authors
// Use of this source code is governed by a BSD-style
// Part of source code is from Go fcgi package

package fcgiclient

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const FCGI_LISTENSOCK_FILENO uint8 = 0
const FCGI_HEADER_LEN uint8 = 8
const VERSION_1 uint8 = 1
const FCGI_NULL_REQUEST_ID uint8 = 0
const FCGI_KEEP_CONN uint8 = 1

const (
	FCGI_BEGIN_REQUEST uint8 = iota + 1
	FCGI_ABORT_REQUEST
	FCGI_END_REQUEST
	FCGI_PARAMS
	FCGI_STDIN
	FCGI_STDOUT
	FCGI_STDERR
	FCGI_DATA
	FCGI_GET_VALUES
	FCGI_GET_VALUES_RESULT
	FCGI_UNKNOWN_TYPE
	FCGI_MAXTYPE = FCGI_UNKNOWN_TYPE
)

const (
	FCGI_RESPONDER uint8 = iota + 1
	FCGI_AUTHORIZER
	FCGI_FILTER
)

const (
	FCGI_REQUEST_COMPLETE uint8 = iota
	FCGI_CANT_MPX_CONN
	FCGI_OVERLOADED
	FCGI_UNKNOWN_ROLE
)

const (
	FCGI_MAX_CONNS  string = "MAX_CONNS"
	FCGI_MAX_REQS   string = "MAX_REQS"
	FCGI_MPXS_CONNS string = "MPXS_CONNS"
)

const (
	maxWrite = 65535 // maximum record body
	maxPad   = 255
)

type header struct {
	Version       uint8
	Type          uint8
	Id            uint16
	ContentLength uint16
	PaddingLength uint8
	Reserved      uint8
}

// for padding so we don't have to allocate all the time
// not synchronized because we don't care what the contents are
var pad [maxPad]byte

// WrongRecordFormat is error returned by recordReader if
// output from io is not in format of FCGI record
var ErrInvalidRecord = errors.New("Wrong FCGI-record format")

func (h *header) init(recType uint8, reqId uint16, contentLength int) {
	h.Version = 1
	h.Type = recType
	h.Id = reqId
	h.ContentLength = uint16(contentLength)
	h.PaddingLength = uint8(-contentLength & 7)
}

type record struct {
	h   header
	buf [maxWrite + maxPad]byte
}

// recordsReader reads from io.Reader and
// returns FCGI-records
type recordReceiver struct {
	r io.Reader
}

func newRecordReceiver(r io.Reader) (rr *recordReceiver) {
	rr = &recordReceiver{r}
	return
}

// receiveRecord reads fcgi record from
// recordReader io.Reader and returns
// FCGI-record
func (rReader *recordReceiver) receiveRecord() (*record, error) {
	rec := new(record)
	err := binary.Read(rReader.r, binary.BigEndian, &rec.h)
	if err != nil {
		if err == io.EOF {
			return rec, err
		} else if err == io.ErrUnexpectedEOF {
			err = ErrInvalidRecord
			return rec, err
		}
	}
	if rec.h.Version != 1 {
		err = errors.New("fcgi: invalid header version")
		return rec, err
	}
	n := int(rec.h.ContentLength) + int(rec.h.PaddingLength)
	if _, err = io.ReadFull(rReader.r, rec.buf[:n]); err != nil {
		return rec, err
	}
	return rec, nil
}

// receiveRecords reads everything from io.Reader and
// returns FCGIResponse with all records from the stream
func (rReader *recordReceiver) receiveRecords() (*FCGIResponse, error) {
	response := newFCGIResponse()
	for {
		rec, err := rReader.receiveRecord()
		if err != nil {
			if err == io.EOF { // Ok, we've processed all the stream
				return response, nil
			}
			if err == ErrInvalidRecord { // Oops, wrong format
				return response, err
			}
			return response, err // In case of unexpected error
		}
		switch rec.h.Type {
		case FCGI_STDOUT:
			response.Stdouts = append(response.Stdouts, *rec)
		case FCGI_STDERR:
			response.Stderrs = append(response.Stderrs, *rec)
		case FCGI_END_REQUEST:
			response.EndRequest = *rec
		}

	}
	return response, nil
}

func (r *record) content() []byte {
	return r.buf[:r.h.ContentLength]
}

type FCGIClient struct {
	mutex     sync.Mutex
	rwc       io.ReadWriteCloser
	h         header
	buf       bytes.Buffer
	keepAlive bool
}

func New(h string, args ...interface{}) (fcgi *FCGIClient, err error) {
	var conn net.Conn
	if len(args) != 1 {
		err = errors.New("fcgi: not enough params")
		return
	}
	switch args[0].(type) {
	case int:
		addr := h + ":" + strconv.FormatInt(int64(args[0].(int)), 10)
		conn, err = net.Dial("tcp", addr)
	case string:
		addr := h + ":" + args[0].(string)
		conn, err = net.Dial("unix", addr)
	default:
		err = errors.New("fcgi: we only accept int (port) or string (socket) params.")
	}
	fcgi = &FCGIClient{
		rwc:       conn,
		keepAlive: false,
	}
	return
}

func (this *FCGIClient) writeRecord(recType uint8, reqId uint16, content []byte) (err error) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.buf.Reset()
	this.h.init(recType, reqId, len(content))
	if err := binary.Write(&this.buf, binary.BigEndian, this.h); err != nil {
		return err
	}
	if _, err := this.buf.Write(content); err != nil {
		return err
	}
	if _, err := this.buf.Write(pad[:this.h.PaddingLength]); err != nil {
		return err
	}
	_, err = this.rwc.Write(this.buf.Bytes())
	return err
}

func (this *FCGIClient) writeBeginRequest(reqId uint16, role uint16, flags uint8) error {
	b := [8]byte{byte(role >> 8), byte(role), flags}
	return this.writeRecord(FCGI_BEGIN_REQUEST, reqId, b[:])
}

func (this *FCGIClient) writeEndRequest(reqId uint16, appStatus int, protocolStatus uint8) error {
	b := make([]byte, 8)
	binary.BigEndian.PutUint32(b, uint32(appStatus))
	b[4] = protocolStatus
	return this.writeRecord(FCGI_END_REQUEST, reqId, b)
}

func (client *FCGIClient) writePairs(recType uint8, reqId uint16, pairs map[string]string) error {
	w := newWriter(client, recType, reqId)
	b := make([]byte, 8)
	for k, v := range pairs {
		n := encodeSize(b, uint32(len(k)))
		n += encodeSize(b[n:], uint32(len(v)))
		if _, err := w.Write(b[:n]); err != nil {
			return err
		}
		if _, err := w.WriteString(k); err != nil {
			return err
		}
		if _, err := w.WriteString(v); err != nil {
			return err
		}
	}
	w.Close()
	return nil
}

func readSize(s []byte) (uint32, int) {
	if len(s) == 0 {
		return 0, 0
	}
	size, n := uint32(s[0]), 1
	if size&(1<<7) != 0 {
		if len(s) < 4 {
			return 0, 0
		}
		n = 4
		size = binary.BigEndian.Uint32(s)
		size &^= 1 << 31
	}
	return size, n
}

func readString(s []byte, size uint32) string {
	if size > uint32(len(s)) {
		return ""
	}
	return string(s[:size])
}

func encodeSize(b []byte, size uint32) int {
	if size > 127 {
		size |= 1 << 31
		binary.BigEndian.PutUint32(b, size)
		return 4
	}
	b[0] = byte(size)
	return 1
}

// bufWriter encapsulates bufio.Writer but also closes the underlying stream when
// Closed.
type bufWriter struct {
	closer io.Closer
	*bufio.Writer
}

func (w *bufWriter) Close() error {
	if err := w.Writer.Flush(); err != nil {
		w.closer.Close()
		return err
	}
	return w.closer.Close()
}

func newWriter(c *FCGIClient, recType uint8, reqId uint16) *bufWriter {
	s := &streamWriter{c: c, recType: recType, reqId: reqId}
	w := bufio.NewWriterSize(s, maxWrite)
	return &bufWriter{s, w}
}

// streamWriter abstracts out the separation of a stream into discrete records.
// It only writes maxWrite bytes at a time.
type streamWriter struct {
	c       *FCGIClient
	recType uint8
	reqId   uint16
}

func (w *streamWriter) Write(p []byte) (int, error) {
	nn := 0
	for len(p) > 0 {
		n := len(p)
		if n > maxWrite {
			n = maxWrite
		}
		if err := w.c.writeRecord(w.recType, w.reqId, p[:n]); err != nil {
			return nn, err
		}
		nn += n
		p = p[n:]
	}
	return nn, nil
}

func (w *streamWriter) Close() error {
	// send empty record to close the stream
	return w.c.writeRecord(w.recType, w.reqId, nil)
}

// FCGIClient.Request send request to fastcgi server and
// return FCGIResponse from it (including all the records in it)
func (this *FCGIClient) Request(env map[string]string, reqStr string) (fcgiResponse *FCGIResponse, err error) {

	var reqId uint16 = 1

	err = this.writeBeginRequest(reqId, uint16(FCGI_RESPONDER), 0)
	if err != nil {
		return
	}
	err = this.writePairs(FCGI_PARAMS, reqId, env)
	if err != nil {
		return
	}
	if len(reqStr) > 0 {
		err = this.writeRecord(FCGI_STDIN, reqId, []byte(reqStr))
		if err != nil {
			return
		}
	}

	recReader := newRecordReceiver(this.rwc)
	fcgiResponse, err = recReader.receiveRecords()
	return
}

// Processes http.Request.Header and returns map of string to string
// which we can use in the FCGI request
// For now we support only unique headers
func headers2env(header http.Header) map[string]string {
	env := make(map[string]string)
	for header_name, value := range header {
		header_name := fmt.Sprintf("HTTP_%s", strings.ToUpper(header_name))
		header_name = strings.Replace(header_name, "-", "_", -1)
		env[header_name] = value[0]
	}

	return env
}

// FCGIClient.HTTPRequest - receives HTTP Request and send it to FCGI-server
func (this *FCGIClient) DoHTTPRequest(http_request *http.Request, script_filename string) (*FCGIHTTPResponse, error) {

	env := make(map[string]string)
	env["REQUEST_METHOD"] = http_request.Method
	env["SCRIPT_FILENAME"] = script_filename
	env["SERVER_SOFTWARE"] = "go / fcgiclient"
	env["REMOTE_ADDR"] = http_request.RemoteAddr
	env["SERVER_PROTOCOL"] = http_request.Proto
	env["REQUEST_URI"] = http_request.URL.Path
	env["DOCUMENT_URI"] = http_request.URL.Path
	env["SCRIPT_NAME"] = http_request.URL.Path
	env["QUERY_STRING"] = http_request.URL.RawQuery

	// HTTP Headers processing
	headers_map := headers2env(http_request.Header)
	for k, v := range headers_map {
		env[k] = v
	}

	fcgiResponse, err := this.Request(env, "")
	http_response, err := fcgiResponse.ParseStdouts()
	return http_response, err
}
