package http

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/blitz-frost/io"
	"github.com/blitz-frost/io/msg"
)

// A Client that exchanges data with a set endpoint through HTTP POST.
type Client struct {
	addr string
	cli  *http.Client
}

// ClientMake returns a unsable Client.
// cli may be nil, in which case the default http client is used.
func ClientMake(addr string, cli *http.Client) Client {
	if cli == nil {
		cli = http.DefaultClient
	}
	return Client{
		addr: addr,
		cli:  cli,
	}
}

func (x Client) Writer() (msg.ExchangeWriter, error) {
	return &writer{
		cli: x,
	}, nil
}

// Handler is a bridge between standard http request handling and the msg framework.
//
// The zero value is directly usable.
type Handler struct {
	ert msg.ExchangeReaderTaker
}

// In order to return a http BadRequest, [ert] should return an error when reading, without using the associated response Writer.
// In any other case, a http OK will be returned, as well as any data written by the time [ert.ReaderTake] returns.
func (x *Handler) ReaderChain(ert msg.ExchangeReaderTaker) error {
	x.ert = ert
	return nil
}

func (x *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := x.ert.ReaderTake(reader{
		r: io.ReaderOf(r.Body),
		w: writerResp{w},
	})

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}

// reader is the [msg.ExchangeReader] implementation
type reader struct {
	r msg.Reader
	w msg.Writer
}

// the request body will be closed automatically on ServeHTTP return.
func (x reader) Close() error {
	return nil
}

func (x reader) Read(b []byte) (int, error) {
	return x.r.Read(b)
}

func (x reader) Writer() (msg.Writer, error) {
	return x.w, nil
}

// writer is the [msg.ExchangeWriter] implementation
type writer struct {
	buf bytes.Buffer
	cli Client
}

func (x *writer) Close() error {
	return nil
}

// Reader sends the http request and returns a response reader
func (x *writer) Reader() (msg.Reader, error) {
	resp, err := x.cli.cli.Post(x.cli.addr, "application/octet-stream", &x.buf)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, errors.New("http response status " + resp.Status)
	}

	return io.ReaderOf(resp.Body), nil
}

func (x *writer) Write(b []byte) (int, error) {
	return x.buf.Write(b)
}

type writerResp struct {
	http.ResponseWriter
}

func (x writerResp) Close() error {
	return nil
}

// HandlerCORS wraps h to accept CORS requests from the specified origin.
func HandlerCORS(origin string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			header := w.Header()
			header.Add("access-control-allow-origin", origin)
			header.Add("access-control-allow-method", http.MethodPost)
			header.Add("access-control-allow-headers", "content-type")

			w.Write([]byte("OK"))
		} else {
			w.Header().Add("access-control-allow-origin", origin)
			h.ServeHTTP(w, r)
		}
	})

}
