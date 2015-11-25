package fcgiclient

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

// FCGIResponse - whole answer from FCGI-server
// which includes all stdouts, stderrs, etc
type FCGIResponse struct {
	Stdouts    []record
	Stderrs    []record
	EndRequest record
}

func newFCGIResponse() *FCGIResponse {
	response := new(FCGIResponse)
	response.Stdouts = make([]record, 0, 0)
	response.Stderrs = make([]record, 0, 0)
	return response
}

// ParseStdouts parses response from FCGI-server
// and returns result in FCGIRHTTPesponse object
func (fcgi_response *FCGIResponse) ParseStdouts() (*FCGIHTTPResponse, error) {
	response := newFCGIHTTPResponse()

	// Collecting buffer from all chunks of php response
	buf := new(bytes.Buffer)
	for _, rec := range fcgi_response.Stdouts {
		_, err := buf.Write(rec.buf[:rec.h.ContentLength])
		if err != nil {
			return response, err
		}
	}

	// processing data in buffer
	for {
		// getting headers
		line, err := buf.ReadString('\n')
		if err != nil {
			break
		}
		// removing trailing \r\n
		line = strings.TrimSuffix(line, "\r\n")
		if len(line) != 0 {
			header_arr := strings.SplitN(line, ": ", 2)
			response.Headers[header_arr[0]] = header_arr[1]
			if header_arr[0] == "Status" {
				status_code_str := header_arr[1][:3]
				response.ResponseCode, err = strconv.Atoi(status_code_str)
			}
		} else {
			// end of headers
			break
		}

	}
	if response.ResponseCode == 0 {
		response.ResponseCode = 200
	}
	// only body left
	_, err := io.Copy(response.Body, buf)
	return response, err
}

// Struct which represents
// HTTP-like response from the FCGI-server
// Implements io.ReaderWriter interface for working with data
type FCGIRespBody struct {
	data []byte
}

func newFCGIRespBody() *FCGIRespBody {
	body := make([]byte, 0, 0)
	return &FCGIRespBody{body}
}

// check if body has no data
func (body FCGIRespBody) eof() bool {
	return len(body.data) == 0
}

// Reading one byte from body data
func (body *FCGIRespBody) readByte() byte {
	// this function assumes that eof() check was done before
	b := body.data[0]
	body.data = body.data[1:]
	return b
}

// Implementing io.Reader interface for
// response body data
func (body *FCGIRespBody) Read(p []byte) (n int, err error) {
	if body.eof() {
		err := io.EOF
		return 0, err
	}
	if c := cap(p); c > 0 {
		for n < c {
			p[n] = body.readByte()
			n++
			if body.eof() {
				break
			}
		}
	}
	return
}

// Implementing io.Writer interface for
// response body data
func (body *FCGIRespBody) Write(p []byte) (n int, err error) {
	body.data = append(body.data, p...)
	return len(p), nil
}

type FCGIHTTPResponse struct {
	ResponseCode int
	Headers      map[string]string
	Body         *FCGIRespBody
}

func newFCGIHTTPResponse() *FCGIHTTPResponse {
	response := new(FCGIHTTPResponse)
	response.Body = newFCGIRespBody()
	response.Headers = make(map[string]string)
	return response
}
