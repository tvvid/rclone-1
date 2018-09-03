package fserrors

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var errUseOfClosedNetworkConnection = errors.New("use of closed network connection")

// make a plausible network error with the underlying errno
func makeNetErr(errno syscall.Errno) error {
	return &net.OpError{
		Op:     "write",
		Net:    "tcp",
		Source: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 123},
		Addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		Err: &os.SyscallError{
			Syscall: "write",
			Err:     errno,
		},
	}
}

type myError1 struct {
	Err error
}

func (e myError1) Error() string { return e.Err.Error() }

type myError2 struct {
	Err error
}

func (e *myError2) Error() string {
	if e == nil {
		return "myError2(nil)"
	}
	if e.Err == nil {
		return "myError2{Err: nil}"
	}
	return e.Err.Error()
}

type myError3 struct {
	Err int
}

func (e *myError3) Error() string { return "hello" }

type myError4 struct {
	e error
}

func (e *myError4) Error() string { return e.e.Error() }

type errorCause struct {
	e error
}

func (e *errorCause) Error() string { return fmt.Sprintf("%#v", e) }

func (e *errorCause) Cause() error { return e.e }

func TestCause(t *testing.T) {
	e3 := &myError3{3}
	e4 := &myError4{io.EOF}
	eNil1 := &myError2{nil}
	eNil2 := &myError2{Err: (*myError2)(nil)}
	errPotato := errors.New("potato")
	nilCause1 := &errorCause{nil}
	nilCause2 := &errorCause{(*myError2)(nil)}

	for i, test := range []struct {
		err           error
		wantRetriable bool
		wantErr       error
	}{
		{nil, false, nil},
		{errPotato, false, errPotato},
		{errors.Wrap(errPotato, "potato"), false, errPotato},
		{errors.Wrap(errors.Wrap(errPotato, "potato2"), "potato"), false, errPotato},
		{errUseOfClosedNetworkConnection, false, errUseOfClosedNetworkConnection},
		{makeNetErr(syscall.EAGAIN), true, syscall.EAGAIN},
		{makeNetErr(syscall.Errno(123123123)), false, syscall.Errno(123123123)},
		{eNil1, false, eNil1},
		{eNil2, false, eNil2.Err},
		{myError1{io.EOF}, false, io.EOF},
		{&myError2{io.EOF}, false, io.EOF},
		{e3, false, e3},
		{e4, false, e4},
		{&errorCause{errPotato}, false, errPotato},
		{nilCause1, false, nilCause1},
		{nilCause2, false, nilCause2.e},
	} {
		gotRetriable, gotErr := Cause(test.err)
		what := fmt.Sprintf("test #%d: %v", i, test.err)
		assert.Equal(t, test.wantErr, gotErr, what)
		assert.Equal(t, test.wantRetriable, gotRetriable, what)
	}
}

func TestShouldRetry(t *testing.T) {
	for i, test := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("potato"), false},
		{errors.Wrap(errUseOfClosedNetworkConnection, "connection"), true},
		{io.EOF, true},
		{io.ErrUnexpectedEOF, true},
		{makeNetErr(syscall.EAGAIN), true},
		{makeNetErr(syscall.Errno(123123123)), false},
		{&url.Error{Op: "post", URL: "/", Err: io.EOF}, true},
		{&url.Error{Op: "post", URL: "/", Err: errUseOfClosedNetworkConnection}, true},
		{&url.Error{Op: "post", URL: "/", Err: fmt.Errorf("net/http: HTTP/1.x transport connection broken: %v", fmt.Errorf("http: ContentLength=%d with Body length %d", 100663336, 99590598))}, true},
		{
			errors.Wrap(&url.Error{
				Op:  "post",
				URL: "http://localhost/",
				Err: makeNetErr(syscall.EPIPE),
			}, "potato error"),
			true,
		},
		{
			errors.Wrap(&url.Error{
				Op:  "post",
				URL: "http://localhost/",
				Err: makeNetErr(syscall.Errno(123123123)),
			}, "listing error"),
			false,
		},
	} {
		got := ShouldRetry(test.err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test #%d: %v", i, test.err))
	}
}
